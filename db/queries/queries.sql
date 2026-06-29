-- sqlc query definitions. Each `-- name:` block generates a type-safe Go method
-- in internal/db (regenerate with `make sqlc`).

-- Insert or update a station's metadata, keyed by station_id.
-- name: UpsertStation :exec
INSERT INTO stations (station_id, name, river_name, latitude, longitude, masl, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (station_id) DO UPDATE SET
    name       = EXCLUDED.name,
    river_name = EXCLUDED.river_name,
    latitude   = EXCLUDED.latitude,
    longitude  = EXCLUDED.longitude,
    masl       = EXCLUDED.masl,
    updated_at = now();

-- Insert an observation; on conflict, update the value/quality/correction so
-- re-polling overlapping time windows is idempotent.
-- name: InsertObservation :exec
INSERT INTO observations (
    time, station_id, parameter, parameter_name, unit, resolution_time, value, quality, correction
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (station_id, parameter, resolution_time, time) DO UPDATE SET
    value      = EXCLUDED.value,
    quality    = EXCLUDED.quality,
    correction = EXCLUDED.correction;

-- name: ListStations :many
SELECT station_id, name, river_name, latitude, longitude, masl, updated_at
FROM stations
ORDER BY station_id;

-- Most recent observation for each parameter at a station (one row per parameter).
-- name: LatestObservations :many
SELECT DISTINCT ON (station_id, parameter)
    time, station_id, parameter, parameter_name, unit, resolution_time, value, quality, correction, ingested_at
FROM observations
WHERE station_id = $1
ORDER BY station_id, parameter, time DESC;

-- Observations for one station and parameter within an inclusive time range,
-- newest first.
-- name: ObservationsByStation :many
SELECT time, station_id, parameter, parameter_name, unit, resolution_time, value, quality, correction, ingested_at
FROM observations
WHERE station_id = $1
  AND parameter = $2
  AND time >= $3
  AND time <= $4
ORDER BY time DESC;
