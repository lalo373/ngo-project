-- Reference DDL (the app applies this automatically via internal/schema).
CREATE TABLE IF NOT EXISTS executive (
    ex_id  INT PRIMARY KEY,
    name   TEXT NOT NULL,
    phone  TEXT NOT NULL UNIQUE,
    sv_id  INT REFERENCES supervisor(sv_id) ON DELETE SET NULL,
    area   TEXT NOT NULL,
    joined TIMESTAMPTZ NOT NULL DEFAULT now()
);
