#!/usr/bin/env bash
# Deploy from your machine to the EC2 instance.
#
#   HOST=ubuntu@ec2-1-2-3-4.compute-1.amazonaws.com ./deploy/deploy.sh
#
# Cross-compiles the Go binary, builds the frontend into static/, syncs
# both to /opt/payroll and restarts the service.
set -euo pipefail
cd "$(dirname "$0")/.."

: "${HOST:?usage: HOST=ubuntu@<ec2-host> ./deploy/deploy.sh}"

echo "==> build frontend"
npm --prefix frontend ci --silent
npm --prefix frontend run build

echo "==> build linux binary"
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/payroll ./cmd

echo "==> upload"
rsync -az --delete dist/payroll "$HOST:/opt/payroll/payroll.new"
rsync -az --delete static/ "$HOST:/opt/payroll/static/"
rsync -az data/ "$HOST:/opt/payroll/data/"

echo "==> activate"
ssh "$HOST" 'sudo mv /opt/payroll/payroll.new /opt/payroll/payroll && \
  sudo chown payroll:payroll /opt/payroll/payroll && \
  sudo chmod 755 /opt/payroll/payroll && \
  sudo systemctl restart payroll && \
  sleep 1 && sudo systemctl is-active payroll'

echo
echo "Deployed. Health: $(ssh "$HOST" 'curl -fsS http://127.0.0.1:5001/healthz')"
