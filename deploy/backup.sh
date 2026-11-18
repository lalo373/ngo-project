#!/usr/bin/env bash
# Nightly PostgreSQL backup (installed by setup.sh, runs from root cron).
# Keeps 14 daily dumps; optionally syncs to S3 when S3_BUCKET is set.
set -euo pipefail
OUT=/var/backups/payroll
mkdir -p "$OUT"
STAMP="$(date +%F)"
FILE="$OUT/payroll-$STAMP.dump"

sudo -u postgres pg_dump -Fc payroll > "$FILE"
gzip -f "$FILE"   # -> payroll-YYYY-MM-DD.dump.gz

find "$OUT" -name 'payroll-*.dump.gz' -mtime +14 -delete

if [[ -n "${S3_BUCKET:-}" ]] && command -v aws >/dev/null; then
  aws s3 cp "$FILE.gz" "s3://${S3_BUCKET}/payroll/" --only-show-errors
fi
