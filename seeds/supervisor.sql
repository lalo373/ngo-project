-- Reference DDL (the app applies this automatically via internal/schema).
CREATE TABLE IF NOT EXISTS supervisor (
    sv_id  INT PRIMARY KEY,
    name   TEXT NOT NULL,
    phone  TEXT NOT NULL UNIQUE,
    dc_id  INT REFERENCES district_coordinator(dc_id) ON DELETE SET NULL,
    area   TEXT NOT NULL,
    joined TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_executive_sv ON executive(sv_id);
