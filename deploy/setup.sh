#!/usr/bin/env bash
# One-time EC2 setup for the all-in-one Payroll deployment.
#
# Run on the EC2 instance (Ubuntu 22.04+), as root:
#   scp -r deploy ubuntu@<ec2-host>:~/deploy
#   ssh ubuntu@<ec2-host> 'sudo bash ~/deploy/setup.sh [your.domain.com]'
#
# Installs: PostgreSQL (local db), Caddy (HTTPS reverse proxy), swap
# file, app user/dirs, systemd service, firewall.
# Idempotent: safe to re-run; existing /etc/payroll/env is preserved.
set -euo pipefail

DOMAIN="${1:-}"
if [[ $EUID -ne 0 ]]; then
  echo "run as root: sudo bash $0 [domain]" >&2
  exit 1
fi

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq postgresql postgresql-contrib caddy ufw rsync curl

echo "==> swap (1 GB RAM needs it)"
if [[ -z "$(swapon --show)" ]]; then
  fallocate -l 2G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile >/dev/null
  swapon /swapfile
  grep -q '^/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
  echo "    2G swap enabled"
else
  echo "    swap already present"
fi

echo "==> app user and directories"
id -u payroll &>/dev/null || useradd --system --home /opt/payroll --shell /usr/sbin/nologin payroll
mkdir -p /opt/payroll/static /opt/payroll/data /etc/payroll /var/backups/payroll
chown -R ubuntu:ubuntu /opt/payroll
chmod 755 /opt/payroll
chown root:root /etc/payroll
chmod 755 /etc/payroll
chown -R postgres:postgres /var/backups/payroll

echo "==> PostgreSQL (local)"
systemctl enable --now postgresql

if [[ ! -f /etc/payroll/env ]]; then
  DBPASS="$(openssl rand -hex 16)"
  ADMINPASS="$(openssl rand -base64 12 | tr -d '=+/')"
  CRONSECRET="$(openssl rand -hex 24)"
  cat > /etc/payroll/env <<EOF
PORT=5001
BIND_ADDR=127.0.0.1
DBURL=postgresql://payroll:${DBPASS}@127.0.0.1:5432/payroll?sslmode=disable
ADMIN_USER=admin
ADMIN_PASS=${ADMINPASS}
CRON_SECRET=${CRONSECRET}
EOF
  chmod 600 /etc/payroll/env
  echo "    wrote /etc/payroll/env"
  echo "    ADMIN_PASS=${ADMINPASS}"
  echo "    CRON_SECRET=${CRONSECRET}"
else
  echo "    /etc/payroll/env exists, keeping it"
  DBPASS="$(grep -oP '(?<=payroll:)[^@]+' /etc/payroll/env || true)"
fi

# role + database (idempotent)
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='payroll'" | grep -q 1; then
  sudo -u postgres psql -q -c "CREATE ROLE payroll LOGIN PASSWORD '${DBPASS}'"
else
  sudo -u postgres psql -q -c "ALTER ROLE payroll WITH PASSWORD '${DBPASS}'"
fi
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='payroll'" | grep -q 1; then
  sudo -u postgres createdb -O payroll payroll
fi

# modest tuning for a 1 GB box
sudo -u postgres psql -q -c "ALTER SYSTEM SET shared_buffers='128MB'"
sudo -u postgres psql -q -c "ALTER SYSTEM SET effective_cache_size='384MB'"
sudo -u postgres psql -q -c "ALTER SYSTEM SET maintenance_work_mem='64MB'"
sudo -u postgres psql -q -c "ALTER SYSTEM SET max_connections='20'"
systemctl restart postgresql

echo "==> systemd service"
install -m 644 "$HERE/payroll.service" /etc/systemd/system/payroll.service
systemctl daemon-reload
systemctl enable payroll

echo "==> Caddy (HTTPS)"
if [[ -z "$DOMAIN" ]]; then
  IP="$(curl -fsS https://checkip.amazonaws.com | tr -d '\n')"
  DOMAIN="${IP//./-}.sslip.io"
  echo "    no domain given, using ${DOMAIN} (resolves to ${IP})"
fi
cat > /etc/caddy/Caddyfile <<EOF
${DOMAIN} {
	encode gzip
	reverse_proxy 127.0.0.1:5001
	header Strict-Transport-Security "max-age=31536000"
}
EOF
systemctl enable --now caddy
systemctl reload caddy || systemctl restart caddy

echo "==> firewall"
ufw allow OpenSSH >/dev/null
ufw allow 80/tcp >/dev/null
ufw allow 443/tcp >/dev/null
ufw --force enable >/dev/null

echo "==> backups"
install -m 755 "$HERE/backup.sh" /usr/local/sbin/payroll-backup
( crontab -l 2>/dev/null | grep -v payroll-backup; echo "30 2 * * * /usr/local/sbin/payroll-backup >> /var/backups/payroll/backup.log 2>&1" ) | crontab -

echo
echo "Setup done."
echo "  App dir:      /opt/payroll (deploy with deploy/deploy.sh from your machine)"
echo "  Env:          /etc/payroll/env (holds ADMIN_PASS, CRON_SECRET, DBURL)"
echo "  URL after deploy: https://${DOMAIN}"
echo "  DB tuning applied; instance has a 2G swap file."
