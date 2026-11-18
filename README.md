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

## Deployment — single EC2 instance (recommended)

One Ubuntu box runs everything: Go binary (API + built SPA), local
PostgreSQL, Caddy for automatic HTTPS. `deploy/` has all scripts.

1. **AWS**: security group allows only 22, 80, 443; attach an **Elastic
   IP** so the address is stable.
2. **One-time setup** (installs Postgres, Caddy, 2G swap, systemd unit,
   firewall, nightly backups; prints generated `ADMIN_PASS`/`CRON_SECRET`):
   ```sh
   scp -r deploy ubuntu@<host>:~/deploy
   ssh ubuntu@<host> 'sudo bash ~/deploy/setup.sh [your.domain.com]'
   ```
   Without a domain it uses `<ip-with-dashes>.sslip.io` — a real DNS
   name with a Let's Encrypt certificate, no domain purchase needed.
3. **Move existing data** from Neon (or skip and seed fresh):
   ```sh
   ssh ubuntu@<host> "sudo NEON_URL='postgresql://...' bash ~/deploy/migrate-from-neon.sh"
   # fresh dummy data instead:
   ssh ubuntu@<host> "sudo bash -c 'set -a; . /etc/payroll/env; set +a; /opt/payroll/payroll -seed /opt/payroll/data/dummy_activities.csv'"
   ```
4. **Deploy / update the app** from your machine:
   ```sh
   HOST=ubuntu@<host> ./deploy/deploy.sh
   ```
5. Open `https://<domain>`. The 20:00 IST job runs in-process; nightly
   `pg_dump` backups land in `/var/backups/payroll` (14-day retention,
   optional `S3_BUCKET` upload).

Config: `PORT`, `BIND_ADDR=127.0.0.1`, `DBURL`, `ADMIN_USER`,
`ADMIN_PASS`, `CRON_SECRET` in `/etc/payroll/env`. Point `DBURL` at Neon
(or any Postgres) if you'd rather not host the database.

### Alternative: Vercel + Render

`vercel.json` (static frontend + API proxy) and `render.yaml`/`Dockerfile`
(API) are kept in the repo if you prefer the managed split instead.

## Security note

`.env` holds real credentials and must never be committed:
`git rm --cached .env`, rotate the Neon password and admin password,
then set the new values in the server's `/etc/payroll/env` (or Render).


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
