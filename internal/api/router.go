// Package api exposes the stored hydrological data over HTTP using chi.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/KealanAU/water-go/internal/db"
	"github.com/KealanAU/water-go/internal/store"
)

type Server struct {
	store *store.Store
	log   *slog.Logger

	defaultPageSize int
	maxPageSize     int
}

// Options configures API behavior such as pagination bounds.
type Options struct {
	DefaultPageSize int
	MaxPageSize     int
}

func NewServer(s *store.Store, log *slog.Logger, opts Options) *Server {
	if opts.MaxPageSize < 1 {
		opts.MaxPageSize = 5000
	}
	if opts.DefaultPageSize < 1 || opts.DefaultPageSize > opts.MaxPageSize {
		opts.DefaultPageSize = opts.MaxPageSize
	}
	return &Server{
		store:           s,
		log:             log,
		defaultPageSize: opts.DefaultPageSize,
		maxPageSize:     opts.MaxPageSize,
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodGet, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(middleware.Timeout(15 * time.Second))

	// Liveness: always 200, no dependencies touched.
	r.Get("/healthz", s.handleLive)
	// Readiness: pings the DB, 503 when unavailable.
	r.Get("/readyz", s.handleReady)

	r.Route("/stations", func(r chi.Router) {
		r.Get("/", s.handleListStations)
		r.Get("/{id}/latest", s.handleLatest)
		r.Get("/{id}/observations", s.handleObservations)
	})
	return r
}

// requestLogger logs one structured line per request via slog.
func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.log.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}

// handleLive is the liveness probe: it reports the process is up without
// touching any dependency, so it stays 200 even when the DB is down.
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReady is the readiness probe: it pings the DB and returns 503 if the
// datastore is unreachable.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListStations(w http.ResponseWriter, r *http.Request) {
	stations, err := s.store.Queries.ListStations(r.Context())
	if err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stations)
}

func (s *Server) handleLatest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rows, err := s.store.Queries.LatestObservations(r.Context(), id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handleObservations(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	param, err := strconv.ParseInt(r.URL.Query().Get("parameter"), 10, 32)
	if err != nil {
		badRequest(w, "parameter query param is required and must be an integer")
		return
	}

	to := time.Now()
	from := to.Add(-7 * 24 * time.Hour)
	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			badRequest(w, "from must be an RFC3339 timestamp")
			return
		}
		from = t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			badRequest(w, "to must be an RFC3339 timestamp")
			return
		}
		to = t
	}
	if to.Before(from) {
		badRequest(w, "to must not be before from")
		return
	}

	limit, offset, err := s.pagination(r)
	if err != nil {
		badRequest(w, err.Error())
		return
	}

	rows, err := s.store.Queries.ObservationsByStation(r.Context(), db.ObservationsByStationParams{
		StationID: id,
		Parameter: int32(param),
		Time:      from,
		Time_2:    to,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// pagination parses and clamps the limit/offset query params. limit defaults to
// the configured page size and is clamped to [1, maxPageSize]; offset defaults
// to 0 and must be non-negative.
func (s *Server) pagination(r *http.Request) (limit, offset int32, err error) {
	limit = int32(s.defaultPageSize)
	if v := r.URL.Query().Get("limit"); v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil {
			return 0, 0, errBadParam("limit must be an integer")
		}
		if n < 1 {
			return 0, 0, errBadParam("limit must be >= 1")
		}
		if n > s.maxPageSize {
			n = s.maxPageSize
		}
		limit = int32(n)
	}

	if v := r.URL.Query().Get("offset"); v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil {
			return 0, 0, errBadParam("offset must be an integer")
		}
		if n < 0 {
			return 0, 0, errBadParam("offset must be >= 0")
		}
		offset = int32(n)
	}
	return limit, offset, nil
}

type badParamError string

func (e badParamError) Error() string { return string(e) }

func errBadParam(msg string) error { return badParamError(msg) }

func (s *Server) serverError(w http.ResponseWriter, err error) {
	s.log.Error("api error", "err", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func badRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
