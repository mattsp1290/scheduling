#!/bin/bash
# Deploy the scheduling web SPA (SvelteKit adapter-static) to the birb.party droplet.
#
# Builds the static site with a SAME-ORIGIN API base (VITE_API_BASE='') so the
# bundle issues relative /api requests, then rsyncs it in place into the
# bind-mounted docroot /var/www/scheduling-web (served by the scheduling-web
# httpd container — no restart needed). Locks against concurrent deploys and
# rolls back in place on failure. Mirrors web-ui/deploy/deploy.sh.
#
# Usage:
#   ./deploy/deploy-web.sh
#   DEPLOY_HOST=birb.party SSH_PORT=2222 ./deploy/deploy-web.sh
set -Eeuo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
log()  { echo -e "${GREEN}[web-deploy]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
err()  { echo -e "${RED}[ERROR]${NC} $1" >&2; exit 1; }

DEPLOY_USER="${DEPLOY_USER:-webapi}"
DEPLOY_HOST="${DEPLOY_HOST:-birb.party}"
SSH_PORT="${SSH_PORT:-2222}"
TARGET="${DEPLOY_USER}@${DEPLOY_HOST}"
REMOTE_DIR="/var/www/scheduling-web"
DOMAIN="scheduling.birb.party"
LOCK_DIR="/var/tmp/scheduling-web-deploy.lock"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WEB_DIR="$(cd "$SCRIPT_DIR/../apps/web" && pwd)"
SSH_CMD=(ssh -p "$SSH_PORT" -o StrictHostKeyChecking=yes)

log "Preflight checks against $TARGET..."
"${SSH_CMD[@]}" "$TARGET" "true" || err "Cannot SSH to $TARGET:$SSH_PORT"

# --- Build static SPA (same-origin API base) --------------------------------
cd "$WEB_DIR"
[ -d node_modules ] || { log "Installing deps..."; npm ci; }
log "Building SPA (VITE_API_BASE='' → same-origin /api)..."
rm -rf build
VITE_API_BASE= npm run build
[ -f build/index.html ] || err "build failed — no build/index.html"
# Guard against the localhost regression: a bundle that baked in the dev
# default would break every visitor's API calls.
if grep -rq "localhost:8080" build/; then err "build leaked localhost:8080 — VITE_API_BASE not applied"; fi
FILES=$(find build -type f | wc -l | tr -d ' ')
log "Built $FILES files"

# --- Lock -------------------------------------------------------------------
BACKUP_DIR=""
cleanup() { "${SSH_CMD[@]}" "$TARGET" "rm -rf $LOCK_DIR" 2>/dev/null || true; }
rollback() {
    warn "Deploy failed — restoring previous docroot in place"
    [ -n "$BACKUP_DIR" ] && "${SSH_CMD[@]}" "$TARGET" "if [ -d $BACKUP_DIR ]; then rsync -a --delete $BACKUP_DIR/ $REMOTE_DIR/; fi" || true
    cleanup
    err "Rolled back."
}
if ! "${SSH_CMD[@]}" "$TARGET" "mkdir $LOCK_DIR 2>/dev/null"; then
    err "Another scheduling-web deploy is in progress ($LOCK_DIR). Remove it if stale."
fi
# Arm traps immediately so a failure between here and the deploy still releases
# the lock (EXIT) and rolls back (ERR/signals). BACKUP_DIR is empty until the
# backup is taken, so an early rollback is a safe no-op.
trap cleanup EXIT
trap rollback ERR INT TERM

"${SSH_CMD[@]}" "$TARGET" "mkdir -p $REMOTE_DIR"
BACKUP_DIR="${REMOTE_DIR}.backup.$(date +%Y%m%d%H%M%S)"

# Back up current docroot (bind-mounted; must update contents in place, not swap the dir)
"${SSH_CMD[@]}" "$TARGET" "if [ -n \"\$(ls -A $REMOTE_DIR 2>/dev/null)\" ]; then cp -r $REMOTE_DIR $BACKUP_DIR; fi"

# --- Deploy in place --------------------------------------------------------
log "Syncing to $TARGET:$REMOTE_DIR..."
rsync -az --delete --delay-updates -e "${SSH_CMD[*]}" build/ "$TARGET:$REMOTE_DIR/"

LOCAL=$(find build -type f | wc -l | tr -d ' ')
REMOTE=$("${SSH_CMD[@]}" "$TARGET" "find $REMOTE_DIR -type f | wc -l | tr -d ' '")
# Explicit rollback (not err): a bare exit fires only the EXIT trap, skipping
# the ERR rollback — call it directly so a mismatch restores the old docroot.
[ "$LOCAL" = "$REMOTE" ] || { warn "file count mismatch local=$LOCAL remote=$REMOTE"; rollback; }
log "Synced $REMOTE files"

# Prune old backups (keep newest)
"${SSH_CMD[@]}" "$TARGET" "find /var/www -maxdepth 1 -name 'scheduling-web.backup.*' -printf '%T@ %p\n' 2>/dev/null | sort -rn | tail -n +2 | cut -d' ' -f2- | xargs -r rm -rf --" || true

trap - ERR INT TERM
cleanup

# --- Verify -----------------------------------------------------------------
EXT=$(curl -s -o /dev/null -w '%{http_code}' --max-time 20 "https://${DOMAIN}/" 2>/dev/null || echo 000)
if [ "$EXT" = "200" ]; then
    log "Verified https://${DOMAIN}/ (HTTP 200)"
else
    warn "https://${DOMAIN}/ returned $EXT — verify DNS + Traefik cert if this is the first deploy"
fi
log "scheduling-web deploy complete → https://${DOMAIN}"
