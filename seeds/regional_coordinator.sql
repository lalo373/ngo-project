-- Reference DDL (the app applies this automatically via internal/schema).
CREATE TABLE IF NOT EXISTS regional_coordinator (
    rc_id  INT PRIMARY KEY,
    name   TEXT NOT NULL,
    phone  TEXT NOT NULL UNIQUE,
    area   TEXT NOT NULL,
    joined TIMESTAMPTZ NOT NULL DEFAULT now()
);
