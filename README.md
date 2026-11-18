# Payroll — Daily Payout System

Go monolith + PostgreSQL (Neon) + TypeScript React SPA. Computes daily
eligibility and payouts for executives, supervisors and district
coordinators; admins manage members and download daily CSV reports.

## Business rules

| Role | Eligibility (computed daily at 20:00 IST) | Daily amount |
|---|---|---|
| Executive | logged in 09:00–17:00 IST **and** target_count ≥ 25 | ₹500 (25–44), ₹1000 (≥ 45) |
| Supervisor | ≥ 70% of their executives eligible | ₹500 |
| District coordinator | ≥ 85% of their supervisors eligible | ₹750 |
| Regional coordinator | rule not defined yet (role exists, no payout) | — |

Amounts and thresholds live in `internal/config/config.go`. Weeks are
Monday–Sunday; the weekly payout is the sum of eligible daily amounts.

## Run

```sh
go run ./cmd              # server on :5001 (reads .env: PORT, DBURL, ADMIN_USER, ADMIN_PASS)
go run ./cmd -seed data/dummy_activities.csv   # seed members + dummy week, then exit
node scripts/generate_dummy_data.mjs           # regenerate the dummy CSV
```

The frontend is built to `static/` and served by the Go binary:

```sh
cd frontend && npm install && npm run dev   # dev server, proxies API to :5001
cd frontend && npm run build                # production build -> ../static
```

## Deployment

The Go API needs a long-running host (Vercel only runs static sites and
short-lived functions), so the app is split in two:

- **Frontend + API proxy on Vercel** — `vercel.json` builds `frontend/`
  into `static/`, serves the SPA, and rewrites `/login`, `/me`,
  `/dailyPayout`, `/weeklyPayouts`, `/calendar` and `/admin/*` to the
  Go API. Cookies stay first-party, so sessions work unchanged.
- **Go API on Render** (Docker) — `render.yaml` + `Dockerfile`.
  The 20:00 IST scheduler runs inside the API process.

### 1. Deploy the API (Render)

1. Render → New → Blueprint → select the repo (`render.yaml` is detected).
2. Set env vars: `DBURL` (Neon URL), `ADMIN_USER`, `ADMIN_PASS`,
   optionally `CRON_SECRET`.
3. Deploy and copy the service URL, e.g. `https://payroll-api.onrender.com`.

### 2. Deploy the frontend (Vercel)

1. Replace every `https://payroll-api.onrender.com` in `vercel.json` with
   your Render URL.
2. Vercel project → Settings → General → **Root Directory must be the
   repo root** (not `frontend`), Framework Preset: *Other*. The build
   command/output directory come from `vercel.json`.
3. Redeploy.

### Keeping the daily job reliable

Free Render instances sleep when idle, which skips the in-process
20:00 IST run. Either use a paid instance, or call the protected
endpoint from any cron service once a day at 14:30 UTC:

```
GET https://<api-host>/cron/runPayout?key=<CRON_SECRET>
```

## Security note

`.env` holds real credentials and must never be committed:
`git rm --cached .env`, rotate the Neon password and admin password,
then set the new values in Render's environment variables.


## Dummy dataset credentials

| Role | IDs | Password |
|---|---|---|
| Executive | 1001–1100 | `exec@123` |
| Supervisor | 301–305 | `super@123` |
| District coordinator | 201 | `dc@123` |
| Regional coordinator | 101 | `rc@123` |
| Admin | username `admin` | `admin@123` (from `.env`) |

Dummy week: 2026-11-16 (Mon) – 2026-11-22 (Sun), 100 executives
(~74% eligible on weekdays, ~40% on weekends so the 70%/85% rules are
visible), 5 supervisors under DC 201.

## API

- `POST /login/{executive|supervisor|district_coordinator|regional_coordinator}` `{id, password}`
- `POST /admin/login` `{username, password}`
- `GET /dailyPayout` · `GET /weeklyPayouts` · `GET /calendar?month=YYYY-MM` (member session)
- `POST /logout` · `GET /me`
- `GET /admin/dashboard/getDataByDate?date=YYYY-MM-DD` — CSV report download
- `POST /admin/runPayout?date=YYYY-MM-DD` — recompute a date on demand
- `GET/POST /admin/members`, `PUT/DELETE /admin/members/{role}/{id}`,
  `PUT /admin/members/{role}/{id}/password`

Sessions are HttpOnly cookies and expire after 24 hours.

## Layout

```
cmd/            entrypoint (server + -seed mode)
internal/api/   HTTP handlers (auth, payouts, admin, SPA serving)
internal/auth/  bcrypt credentials, 24h sessions
internal/payout/eligibility engine + 20:00 IST scheduler (worker pool)
internal/schema/embedded SQL schema, applied at startup
internal/seed/  member + CSV seeding, runs the engine per date
frontend/       Vite React TS app (black & white theme)
static/         built frontend served by the Go binary
```
