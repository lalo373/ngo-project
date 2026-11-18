-- Payroll schema. Applied automatically at startup (idempotent).
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS regional_coordinator (
    rc_id  INT PRIMARY KEY,
    name   TEXT NOT NULL,
    phone  TEXT NOT NULL UNIQUE,
    area   TEXT NOT NULL,
    joined TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS district_coordinator (
    dc_id INT PRIMARY KEY,
    name  TEXT NOT NULL,
    phone TEXT NOT NULL UNIQUE,
    rc_id INT REFERENCES regional_coordinator(rc_id) ON DELETE SET NULL,
    area  TEXT NOT NULL,
    joined TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS supervisor (
    sv_id INT PRIMARY KEY,
    name  TEXT NOT NULL,
    phone TEXT NOT NULL UNIQUE,
    dc_id INT REFERENCES district_coordinator(dc_id) ON DELETE SET NULL,
    area  TEXT NOT NULL,
    joined TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS executive (
    ex_id INT PRIMARY KEY,
    name  TEXT NOT NULL,
    phone TEXT NOT NULL UNIQUE,
    sv_id INT REFERENCES supervisor(sv_id) ON DELETE SET NULL,
    area  TEXT NOT NULL,
    joined TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_executive_sv  ON executive(sv_id);
CREATE INDEX IF NOT EXISTS idx_supervisor_dc ON supervisor(dc_id);

-- Hashed login credentials for every member (passwords given by admin).
CREATE TABLE IF NOT EXISTS credential (
    role          TEXT NOT NULL CHECK (role IN ('executive','supervisor','district_coordinator','regional_coordinator')),
    login_id      INT  NOT NULL,
    password_hash TEXT NOT NULL,
    PRIMARY KEY (role, login_id)
);

-- Admin account.
CREATE TABLE IF NOT EXISTS app_user (
    username      TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL
);

-- Session tokens; all sessions expire after 24 hours.
CREATE TABLE IF NOT EXISTS session (
    token      TEXT PRIMARY KEY,
    role       TEXT NOT NULL,
    login_id   TEXT NOT NULL, -- username for admin, member id as text for members
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_session_expiry ON session(expires_at);

-- Daily raw executive activity (normally fed by an external system;
-- dummy CSV data is used for now, see data/dummy_activities.csv).
CREATE TABLE IF NOT EXISTS daily_activity (
    activity_date DATE NOT NULL,
    ex_id         INT  NOT NULL REFERENCES executive(ex_id) ON DELETE CASCADE,
    login_time    TIMESTAMPTZ,
    target_count  INT  NOT NULL DEFAULT 0,
    PRIMARY KEY (activity_date, ex_id)
);

-- One row per member per day, computed at 20:00 IST by the payout engine.
CREATE TABLE IF NOT EXISTS payout (
    payout_date DATE           NOT NULL,
    role        TEXT           NOT NULL,
    login_id    INT            NOT NULL,
    eligible    BOOLEAN        NOT NULL,
    amount      NUMERIC(10,2)  NOT NULL DEFAULT 0,
    metric      NUMERIC(7,2),  -- executives: target_count; supervisors/DCs: eligible percent
    computed_at TIMESTAMPTZ    NOT NULL,
    PRIMARY KEY (payout_date, role, login_id)
);
CREATE INDEX IF NOT EXISTS idx_payout_member ON payout(role, login_id, payout_date DESC);
CREATE INDEX IF NOT EXISTS idx_payout_date   ON payout(payout_date);
