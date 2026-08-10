# Shipyard

Open-source software delivery platform: CI/CD pipelines, distributed runners, artifact/package hosting, and OCI registry in one product.

```text
source → pipeline → job → artifact/package/image → release → deployment
```

## Status

Phases 0–9 foundation verticals are implemented locally on `master` (not pushed).

| Phase | Capability |
|---|---|
| 0 | Server, Postgres migrations, storage (filesystem + S3), UI shell, Compose |
| 1 | Users/auth/sessions/tokens, OIDC start/callback, orgs, projects, RBAC, audit |
| 2 | `shipyard.yml` parse/DAG, pipeline runs/jobs/steps, run detail UI |
| 3 | Runner registration, leasing, shell executor, logs (secret masking) |
| 4 | Artifact upload/download with digests + provenance links |
| 5 | OCI Distribution `/v2` + Bearer/Basic token auth, BuildKit helper |
| 6 | Environments, releases, deployments |
| 7–8 | Multi-runner labels + DB lease fencing / cluster node heartbeat |
| 9 | Generic + npm/Maven repository protocols |

CLI: `go run ./cmd/shipyard` (`login`, `whoami`, `status`, `run`).

## Quick start

```bash
# API (requires Postgres)
export SHIPYARD_DATABASE_URL=postgres://shipyard:shipyard@localhost:5432/shipyard?sslmode=disable
go run ./cmd/shipyard-server

# UI
yarn install
yarn workspace @shipyard/web dev

# Runner (after creating a registration token)
SHIPYARD_URL=http://127.0.0.1:8080 \
SHIPYARD_REGISTRATION_TOKEN=... \
go run ./cmd/shipyard-runner
```

Compose:

```bash
docker compose -f deploy/compose/compose.yml up --build
```

## Repository layout

| Path | Purpose |
|---|---|
| `cmd/shipyard-server` | Control plane |
| `cmd/shipyard-runner` | Job executor |
| `internal/` | Domain packages |
| `migrations/` | PostgreSQL migrations |
| `apps/web` | Operator UI |
| `packages/ui` | `@shipyard/ui` |
| `deploy/compose` | Standalone stack |
| `DESIGN.md` | Normative UI contract |

## Normative docs

1. `DESIGN.md`
2. `docs/architecture/`
3. `docs/handoff/`
