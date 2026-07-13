#!/bin/bash
# Deploy the scheduling API (Go binary) to the birb.party droplet.
#
# Builds a static linux/amd64 binary, backs up the current binary + a SQLite
# snapshot, swaps the binary on the webapi-owned /opt/scheduling, restarts the
# scheduling-api systemd unit, and health-checks /api/health. Rolls the binary
# back on health-check failure.
#
# The scheduling-api unit, /opt/scheduling dirs, .env, and the sudoers Cmnd for
# `systemctl restart scheduling-api` are installed ONCE at provision time
# (web-api/deploy/setup-server.sh) or via web-api/deploy/scheduling-bootstrap.sh
# on an already-provisioned droplet. This script does NOT create them.
#
# Usage:
#   ./deploy/deploy-api.sh
#   DEPLOY_HOST=birb.party SSH_PORT=2222 ./deploy/deploy-api.sh
set -Eeuo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
log()  { echo -e "${GREEN}[api-deploy]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
err()  { echo -e "${RED}[ERROR]${NC} $1" >&2; exit 1; }

DEPLOY_USER="${DEPLOY_USER:-webapi}"
DEPLOY_HOST="${DEPLOY_HOST:-birb.party}"
SSH_PORT="${SSH_PORT:-2222}"
DEPLOY_DIR="/opt/scheduling"
BACKUP_DIR="/opt/scheduling-backup"
BINARY="scheduling-api"
TARGET="${DEPLOY_USER}@${DEPLOY_HOST}"
LOCK_DIR="/var/tmp/scheduling-api-deploy.lock"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

ssh_cmd() { ssh -p "$SSH_PORT" -o StrictHostKeyChecking=yes "$TARGET" "$@"; }

# --- Preflight (fail fast rather than hang on a sudo password prompt) --------
log "Preflight checks against $TARGET..."
ssh_cmd "true" || err "Cannot SSH to $TARGET:$SSH_PORT"
ssh_cmd "test -d $DEPLOY_DIR" || err "$DEPLOY_DIR missing on host — run web-api/deploy/scheduling-bootstrap.sh first"
ssh_cmd "test -f $DEPLOY_DIR/.env" || err "$DEPLOY_DIR/.env missing — bootstrap not run"
ssh_cmd "sudo -n systemctl status scheduling-api >/dev/null 2>&1 || sudo -nl /usr/bin/systemctl restart scheduling-api >/dev/null 2>&1" \
    || err "webapi cannot 'sudo -n systemctl restart scheduling-api' — sudoers not provisioned (bootstrap not run)"

# --- Build ------------------------------------------------------------------
log "Building $BINARY (linux/amd64, static, GOWORK=off)..."
cd "$PROJECT_ROOT/apps/api"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOWORK=off go build -trimpath -ldflags "-s -w" -o "/tmp/$BINARY.$$" ./cmd/api
BUILT="/tmp/$BINARY.$$"
trap 'rm -f "$BUILT"' EXIT
[ -f "$BUILT" ] || err "build produced no binary"
log "Built $(du -h "$BUILT" | cut -f1) binary"

# --- Lock -------------------------------------------------------------------
if ! ssh_cmd "mkdir $LOCK_DIR 2>/dev/null"; then
    err "Another scheduling-api deploy is in progress ($LOCK_DIR). Remove it if stale."
fi
release_lock() { ssh_cmd "rmdir $LOCK_DIR 2>/dev/null" || true; }
trap 'rm -f "$BUILT"; release_lock' EXIT INT TERM

# --- Backup current binary + SQLite snapshot (rotate 5) ---------------------
log "Backing up current binary and DB snapshot..."
ssh_cmd "bash -s" <<'REMOTE'
set -e
DEPLOY_DIR="/opt/scheduling"; BACKUP_DIR="/opt/scheduling-backup"
TS=$(date +%Y%m%d_%H%M%S)
mkdir -p "$BACKUP_DIR"
if [ -f "$DEPLOY_DIR/scheduling-api" ]; then
    cp "$DEPLOY_DIR/scheduling-api" "$BACKUP_DIR/scheduling-api_$TS"
    ls -t "$BACKUP_DIR"/scheduling-api_* 2>/dev/null | tail -n +6 | xargs -r rm -f
fi
DB="$DEPLOY_DIR/data/scheduling.db"
if [ -f "$DB" ]; then
    SNAP="$BACKUP_DIR/scheduling.db_$TS"
    # Prefer the SQLite online-backup API (consistent under a live writer);
    # fall back to cp (single-writer app, brief window) if sqlite3 is absent.
    if command -v sqlite3 >/dev/null 2>&1; then
        sqlite3 "$DB" ".backup '$SNAP'"
    else
        cp "$DB" "$SNAP"
    fi
    ls -t "$BACKUP_DIR"/scheduling.db_* 2>/dev/null | tail -n +6 | xargs -r rm -f
    echo "DB snapshot: $SNAP"
fi
REMOTE

# --- Swap binary + restart --------------------------------------------------
log "Uploading and swapping binary..."
scp -P "$SSH_PORT" -o StrictHostKeyChecking=yes "$BUILT" "$TARGET:/tmp/$BINARY.new"
ssh_cmd "mv /tmp/$BINARY.new $DEPLOY_DIR/$BINARY && chmod 755 $DEPLOY_DIR/$BINARY"

log "Restarting scheduling-api..."
ssh_cmd "sudo -n systemctl restart scheduling-api"

# --- Health check (local loopback, then external HTTPS) ---------------------
health_ok() {
    sleep 3
    for i in $(seq 1 5); do
        code=$(ssh_cmd "curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8090/api/health" || echo 000)
        [ "$code" = "200" ] && return 0
        warn "local health attempt $i/5 got $code"; sleep 3
    done
    return 1
}
if health_ok; then
    log "Local health check passed (127.0.0.1:8090/api/health)"
else
    warn "Health check FAILED — rolling back to previous binary"
    ssh_cmd "bash -s" <<'REMOTE'
set -e
BACKUP_DIR="/opt/scheduling-backup"; DEPLOY_DIR="/opt/scheduling"
LATEST=$(ls -t "$BACKUP_DIR"/scheduling-api_* 2>/dev/null | head -1)
if [ -n "$LATEST" ]; then cp "$LATEST" "$DEPLOY_DIR/scheduling-api" && chmod 755 "$DEPLOY_DIR/scheduling-api"; fi
sudo -n systemctl restart scheduling-api || true
REMOTE
    err "Deploy failed health check; rolled back. Check: ssh $TARGET 'sudo journalctl -u scheduling-api -n 100 --no-pager'"
fi

# External check (DNS + Traefik + TLS). Non-fatal warning if DNS/cert not ready.
EXT=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 "https://scheduling.birb.party/api/health" 2>/dev/null || echo 000)
if [ "$EXT" = "200" ]; then
    log "External health check passed (https://scheduling.birb.party/api/health)"
else
    warn "External check returned $EXT — verify DNS record + Traefik cert for scheduling.birb.party"
fi

log "scheduling-api deploy complete."
