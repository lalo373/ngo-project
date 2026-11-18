-- Reference DDL (the app applies this automatically via internal/schema).
CREATE TABLE IF NOT EXISTS district_coordinator (
    dc_id  INT PRIMARY KEY,
    name   TEXT NOT NULL,
    phone  TEXT NOT NULL UNIQUE,
    rc_id  INT REFERENCES regional_coordinator(rc_id) ON DELETE SET NULL,
    area   TEXT NOT NULL,
    joined TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_supervisor_dc ON supervisor(dc_id);
