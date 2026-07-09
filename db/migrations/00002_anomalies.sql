-- Anomaly records produced by the analytics stage. Statements are idempotent
-- to match the observations table pattern and the goose runner's boot-time
-- apply.

-- +goose Up
CREATE TABLE IF NOT EXISTS anomalies (
    time            TIMESTAMPTZ      NOT NULL,
    station_id      TEXT             NOT NULL REFERENCES stations(station_id),
    parameter       INTEGER          NOT NULL,
    parameter_name  TEXT             NOT NULL,
    value           DOUBLE PRECISION NOT NULL,
    mean            DOUBLE PRECISION NOT NULL,
    stddev          DOUBLE PRECISION NOT NULL,
    zscore          DOUBLE PRECISION NOT NULL,
    threshold       DOUBLE PRECISION NOT NULL,
    detected_at     TIMESTAMPTZ      NOT NULL DEFAULT now(),
    -- Enables idempotent upserts.
    PRIMARY KEY (station_id, parameter, time)
);

SELECT create_hypertable('anomalies', 'time', if_not_exists => TRUE);

-- Supports the API's recent-anomalies-per-station lookup.
CREATE INDEX IF NOT EXISTS anomalies_station_param_idx
    ON anomalies (station_id, parameter, time DESC);

-- +goose Down
DROP TABLE IF EXISTS anomalies;
