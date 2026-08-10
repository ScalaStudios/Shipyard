# Deploying Shipyard

How to install, configure, and put Shipyard behind **Caddy** (preferred) or **nginx**.

## What you get

| Piece | Default |
|---|---|
| Control plane API | `:8080` |
| Operator UI (dev) | `:5173` (Vite) or served by reverse proxy |
| PostgreSQL | `:5432` |
| Public URL | `https://shipyard.example.com` |

Chain: `source → pipeline → job → artifact/package/image → release → deployment`

## Fastest path (install script)

From a Linux host with Docker (or rootless Docker):

```bash
# interactive — prompts for domain + proxy
./scripts/install.sh

# non-interactive example (Caddy)
DOMAIN=shipyard.example.com \
EMAIL=ops@example.com \
PROXY=caddy \
./scripts/install.sh --yes
```

The script will:

1. Check Docker / Compose
2. Write `.env` from `.env.example` (secrets, public URL)
3. Start Postgres + `shipyard-server` via Compose
4. Install Bun deps and optionally run the UI
5. Drop a **Caddyfile** or **nginx** site config under `deploy/`
6. Print next steps (first user, runners, Discord, forge webhooks)

## Manual Compose

```bash
cp .env.example .env
# edit SHIPYARD_PUBLIC_URL, SHIPYARD_WEBHOOK_SECRET, SHIPYARD_SECRETS_KEY, …

export PATH="$HOME/bin:$PATH"   # rootless Docker on Arch
export DOCKER_HOST=unix:///run/user/$UID/docker.sock

docker compose -f deploy/compose/compose.yml up --build -d
curl -sf http://127.0.0.1:8080/healthz
```

UI (development):

```bash
bun install
SHIPYARD_PUBLIC_URL=http://127.0.0.1:5173 bun run dev
```

Open `http://127.0.0.1:5173` — first visit can register when the user table is empty.

## Reverse proxy (Caddy preferred)

Caddy auto-HTTPS when DNS points at the host.

```bash
# after install.sh, or copy manually:
sudo cp deploy/caddy/Caddyfile /etc/caddy/Caddyfile
# set {$SHIPYARD_DOMAIN} or edit the site address
sudo systemctl reload caddy
```

Example `deploy/caddy/Caddyfile`:

- Terminates TLS
- Serves `/` → Vite preview or static UI build (or proxies to `:5173` in lab)
- Proxies `/api`, `/v2`, `/auth`, `/repository`, `/healthz`, `/readyz` → `:8080`

### nginx

```bash
sudo cp deploy/nginx/shipyard.conf /etc/nginx/sites-available/shipyard
sudo ln -s /etc/nginx/sites-available/shipyard /etc/nginx/sites-enabled/shipyard
# set server_name + TLS paths
sudo nginx -t && sudo systemctl reload nginx
```

Use Certbot or your own certificates. Prefer Caddy when you can.

## First-time operator setup

1. Open the UI → **Create first account** (allowed when no users exist, or `SHIPYARD_ALLOW_REGISTER=true`)
2. **Projects** → create org + project
3. **Pipelines** → save a pipeline → **Run**
4. **Runners** → **Generate install command** → copy the `curl | bash` one-liner onto a host (Docker or Go). Same idea as Pterodactyl Wings.
5. **Settings → Integrations** → connect GitHub / Forgejo / GitLab / …
6. Point forge webhooks at  
   `https://YOUR_DOMAIN/api/v1/webhooks/{provider}?connection_id=…`
7. **Settings → Notifications** → Discord webhook or bot (Sentry-style embeds)

## Important environment variables

| Variable | Purpose |
|---|---|
| `SHIPYARD_DATABASE_URL` | Postgres DSN |
| `SHIPYARD_PUBLIC_URL` | Links in bot comments + Discord embeds |
| `SHIPYARD_API_URL` | API origin embedded in runner `curl \| bash` install scripts |
| `SHIPYARD_WEBHOOK_SECRET` | Global webhook HMAC / shared secret fallback |
| `SHIPYARD_SECRETS_KEY` | Base64 32-byte key for encrypted secrets |
| `SHIPYARD_ALLOW_REGISTER` | Open registration (otherwise first-user only) |
| `SHIPYARD_OIDC_*` | Login presets (GitHub, GitLab, Forgejo, Gitea, generic) |

See `.env.example` for the full list.

## Production notes

- Put Postgres on a volume; back it up
- Set strong `SHIPYARD_SECRETS_KEY` and `SHIPYARD_WEBHOOK_SECRET`
- Set `SHIPYARD_ALLOW_REGISTER=false` after bootstrap
- Prefer filesystem or S3 for blobs (`SHIPYARD_STORAGE_BACKEND`)
- Run at least one runner with labels matching your jobs
- Keep `SHIPYARD_PUBLIC_URL` on the **HTTPS** origin users hit

## Smoke test

```bash
./scripts/smoke.sh
```

## Troubleshooting

| Symptom | Check |
|---|---|
| UI can’t reach API | Proxy `/api` → `:8080`; Vite proxy in `apps/web/vite.config.ts` for local |
| Webhooks 401 | Connection webhook secret vs forge signature headers |
| Discord silent | **Send test** in Settings; confirm webhook URL / bot intents |
| Runner idle | Registration token, labels, server URL reachable from runner |
| Migrations fail | Postgres healthy; `SHIPYARD_MIGRATIONS_DIR` mounted in Compose |
