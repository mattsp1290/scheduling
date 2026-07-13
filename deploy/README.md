# Deploying scheduling to https://scheduling.birb.party

The scheduling app runs on the **same DigitalOcean droplet** as birb.party,
reusing its rootless-Docker + Traefik + systemd infrastructure (owned by the
`web-api` repo). Layout on the droplet:

| Component        | What                                   | Where / port |
|------------------|----------------------------------------|--------------|
| `scheduling-api` | Go binary, systemd unit, `User=webapi` | `127.0.0.1:8090`, DB at `/opt/scheduling/data/scheduling.db` |
| `scheduling-web` | SvelteKit adapter-static SPA (httpd)   | container publishes `0.0.0.0:8084`, docroot `/var/www/scheduling-web` |
| Traefik routing  | `web-api/traefik/dynamic/scheduling.birb.party.yml` | separate LE cert |

Same-origin: the browser loads the SPA from `https://scheduling.birb.party/`
and calls the API at `https://scheduling.birb.party/api/*` (Traefik routes
`/api/*` to `scheduling-api`, everything else to `scheduling-web`).

## One-time setup (root, per droplet)

A new privileged service (systemd unit + sudoers Cmnd + docroot) can only be
installed by root — the scoped deploy user cannot. Two ways:

1. **Fresh provision / rebuild** — `web-api/deploy/setup-server.sh` now installs
   everything (dirs, unit, `.env`, sudoers, `scheduling-web` container). Nothing
   extra to do.
2. **Existing droplet** — run the idempotent bootstrap from a `web-api`
   checkout, **as a root-capable user** (the scoped `webapi` user cannot install
   a unit or edit sudoers on an already-provisioned box):
   ```bash
   cd web-api
   DEPLOY_HOST=birb.party SSH_PORT=2222 BOOTSTRAP_USER=root ./deploy/scheduling-bootstrap.sh
   ```
   Set `BOOTSTRAP_USER` to a full-sudo admin if root SSH is disabled (it uses
   `sudo` only when the connecting user isn't root). It creates the dirs +
   `.env`, installs & enables the unit, appends the sudoers Cmnds (validated with
   `visudo -c` before replacing the file), drops the httpd + Traefik configs, and
   starts `scheduling-web` via webapi's rootless docker.

Also ensure the DNS record exists (Terraform), required **before** Traefik can
obtain the cert:
```bash
cd infrastructure/terraform && terraform apply   # adds digitalocean_record.scheduling
```
(or add the `scheduling` A record → droplet IP manually in the DO console).

## Routine deploys (unprivileged)

From the `scheduling` repo root, after the one-time setup:
```bash
./deploy/deploy-api.sh    # build + backup (binary + SQLite snapshot) + restart + health-check + rollback
./deploy/deploy-web.sh    # build static SPA (same-origin) + in-place rsync + verify
```
Both take `DEPLOY_HOST` / `SSH_PORT` overrides (defaults `birb.party` / `2222`).

## Verifying

```bash
curl -sS https://scheduling.birb.party/api/health        # 200 JSON
curl -sSI https://scheduling.birb.party/                  # 200 + valid LE cert
curl -sSI https://scheduling.birb.party/surveys/new       # 200 (SPA fallback, not 404)
```
Then exercise: sign up → create survey → open the public `/s/<token>` link →
submit availability → view results. The session cookie must be
`Secure; HttpOnly; SameSite=Lax` on `scheduling.birb.party`.

## Notes / caveats

- **Cross-repo coupling (intentional):** the container/Traefik/unit configs live
  in `web-api` (the droplet-infra repo), because Traefik watches only
  `/opt/webapi/traefik/dynamic` and web-api's `ensure_deps` rsyncs `traefik/`
  and `scheduling-httpd/` with `--delete`. Changing scheduling's *routing* or
  *httpd* config requires a **web-api** deploy; changing the *app* uses the
  scripts here.
- **Data durability:** the SQLite DB is on droplet local disk. `deploy-api.sh`
  snapshots it before each deploy (rotated in `/opt/scheduling-backup`), but a
  droplet **rebuild wipes `/opt`**. Continuous backup (litestream → DO Spaces)
  is tracked separately — until then, a repave resets all scheduling data.
- **`ALLOWED_ORIGINS` is load-bearing**, not defensive: the API returns 403 for
  same-origin POSTs whose `Origin` is not exactly `https://scheduling.birb.party`.
- **web-api and this repo must advance together.** web-api's `ensure_deps` rsyncs
  `traefik/` and `scheduling-httpd/` with `--delete`, so deploying web-api from a
  checkout that predates the scheduling configs would remove them from the
  droplet (Traefik drops the routes; `scheduling-web` loses its bind source).
  Keep both repos current before a web-api deploy.
- **A droplet rebuild leaves scheduling down until you re-deploy.** `setup-server.sh`
  enables `scheduling-api` but does not ship its binary, and the SPA docroot is
  empty after a repave. After any rebuild, re-run **both** `./deploy/deploy-api.sh`
  and `./deploy/deploy-web.sh` from this repo (and restore the DB — see above).
- **First-run CSP:** the SPA's CSP is a `<meta>` baked into `index.html` (script
  locked to `self` + bootstrap hash; styles unrestricted so Svelte transitions
  work). After the first deploy, open the app in a browser and confirm no CSP
  violations in the console.
