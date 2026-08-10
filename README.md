# Shipyard

Open-source software delivery platform: CI/CD pipelines, distributed runners, artifact/package hosting, and OCI registry in one product.

```text
source → pipeline → job → artifact/package/image → release → deployment
```

<p align="center">
  <img src="docs/brand/shipyard-mark.svg" alt="Shipyard mark" width="96" height="96" />
</p>

## Install & deploy

**Preferred path** (Compose + Bun + Caddy config):

```bash
./scripts/install.sh
# or
DOMAIN=shipyard.example.com PROXY=caddy ./scripts/install.sh --yes
```

Full guide: **[docs/DEPLOY.md](docs/DEPLOY.md)** (Caddy preferred, nginx included, forge webhooks, Discord, runners).

## Quick start (dev)

```bash
# API (Compose Postgres + server)
export PATH="$HOME/bin:$PATH"
export DOCKER_HOST=unix:///run/user/$UID/docker.sock   # rootless Docker
docker compose -f deploy/compose/compose.yml up --build -d

# UI
bun install
bun run dev
```

Open http://127.0.0.1:5173 — create the first account when prompted.

```bash
./scripts/smoke.sh   # API smoke (server must be up)
```

## What ships

| Area | Capability |
|---|---|
| Auth | Local users, sessions, PATs, OIDC (GitHub / GitLab / Forgejo / Gitea) |
| CI | Pipelines, runs, jobs, logs, runners |
| Delivery | Artifacts, packages (npm/Maven), OCI `/v2`, releases, deployments |
| Forges | Webhooks + PR/MR bot comments |
| Alerts | In-app notifications + Discord embeds |
| Ops | Cluster heartbeat, secrets vault, install script + reverse proxies |

## Repository layout

| Path | Purpose |
|---|---|
| `cmd/shipyard-server` | Control plane |
| `cmd/shipyard-runner` | Job executor |
| `cmd/shipyard` | CLI |
| `apps/web` | Operator UI |
| `packages/ui` | `@shipyard/ui` |
| `deploy/compose` | Standalone stack |
| `deploy/caddy` | Caddy reverse proxy (preferred) |
| `deploy/nginx` | nginx reverse proxy |
| `scripts/install.sh` | One-shot installer |
| `DESIGN.md` | Normative UI contract |
| `docs/DEPLOY.md` | Deploy / setup guide |

## Normative docs

1. `DESIGN.md`
2. `docs/DEPLOY.md`
3. `docs/architecture/`
4. `AGENTS.md`
