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

	"github.com/kealanclarke/water-go/internal/db"
	"github.com/kealanclarke/water-go/internal/store"
)

// Server holds dependencies for the HTTP API.
type Server struct {
	store *store.Store
	log   *slog.Logger
}

// NewServer builds the API server.
func NewServer(s *store.Store, log *slog.Logger) *Server {
	return &Server{store: s, log: log}
}

// Routes returns the configured chi router.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Get("/healthz", s.handleHealth)
	r.Route("/stations", func(r chi.Router) {
		r.Get("/", s.handleListStations)
		r.Get("/{id}/latest", s.handleLatest)
		r.Get("/{id}/observations", s.handleObservations)
	})
	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parameter query param is required"})
		return
	}

	to := time.Now()
	from := to.Add(-7 * 24 * time.Hour)
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}

	rows, err := s.store.Queries.ObservationsByStation(r.Context(), db.ObservationsByStationParams{
		StationID: id,
		Parameter: int32(param),
		Time:      from,
		Time_2:    to,
	})
	if err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) serverError(w http.ResponseWriter, err error) {
	s.log.Error("api error", "err", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
