-- TimescaleDB schema for NVE hydrological time-series.
-- Applied at startup by the store's goose migration runner; statements are idempotent.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS stations (
    station_id   TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    river_name   TEXT,
    latitude     DOUBLE PRECISION,
    longitude    DOUBLE PRECISION,
    masl         DOUBLE PRECISION,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS observations (
    time            TIMESTAMPTZ      NOT NULL,
    station_id      TEXT             NOT NULL REFERENCES stations(station_id),
    parameter       INTEGER          NOT NULL,
    parameter_name  TEXT             NOT NULL,
    unit            TEXT             NOT NULL,
    resolution_time INTEGER          NOT NULL,
    value           DOUBLE PRECISION,
    quality         INTEGER,
    correction      INTEGER,
    ingested_at     TIMESTAMPTZ      NOT NULL DEFAULT now(),
    -- One row per station/parameter/resolution/timestamp; enables idempotent upserts.
    PRIMARY KEY (station_id, parameter, resolution_time, time)
);

SELECT create_hypertable('observations', 'time', if_not_exists => TRUE);

-- Supports the API's per-station, per-parameter time-range and latest lookups.
CREATE INDEX IF NOT EXISTS observations_station_param_idx
    ON observations (station_id, parameter, time DESC);

-- +goose Down
DROP TABLE IF EXISTS observations;
DROP TABLE IF EXISTS stations;
