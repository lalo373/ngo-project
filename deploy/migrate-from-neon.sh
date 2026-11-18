#!/usr/bin/env bash
# One-time migration: copy your existing Neon database into the local
# PostgreSQL on the EC2 box, so members, activities and payouts carry over.
#
# Run on the EC2 instance:
#   NEON_URL='postgresql://...' bash migrate-from-neon.sh
set -euo pipefail

: "${NEON_URL:?set NEON_URL to the current Neon connection string}"
LOCAL_URL="$(grep -oP '(?<=^DBURL=).*' /etc/payroll/env | tr -d '"')"

echo "==> dumping Neon"
pg_dump "$NEON_URL" --no-owner --no-privileges --clean --if-exists | gzip > /tmp/neon.dump.gz

echo "==> restoring locally"
gunzip -c /tmp/neon.dump.gz | psql "$LOCAL_URL" -v ON_ERROR_STOP=1 >/dev/null

echo "==> row counts"
psql "$LOCAL_URL" -c "SELECT
  (SELECT count(*) FROM executive) AS executives,
  (SELECT count(*) FROM supervisor) AS supervisors,
  (SELECT count(*) FROM daily_activity) AS activities,
  (SELECT count(*) FROM payout) AS payouts;"

rm -f /tmp/neon.dump.gz
echo "Done. The local DB now mirrors Neon."
