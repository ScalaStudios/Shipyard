# Shipyard

Open-source software delivery platform: CI/CD pipelines, distributed runners, artifact/package hosting, and OCI registry in one product.

```text
source → pipeline → job → artifact/package/image → release → deployment
```

## Status

Phase 0 — Foundation. See [issues](https://git.lunarlabs.dev/Shipyard/shipyard/issues).

## Repository

| Path | Purpose |
|---|---|
| `cmd/shipyard-server` | Control-plane binary |
| `cmd/shipyard-runner` | Runner binary (later phases) |
| `internal/` | Go domain packages |
| `migrations/` | PostgreSQL migrations |
| `apps/web` | Operator UI (Vite + React) |
| `packages/ui` | `@shipyard/ui` design system |
| `deploy/compose` | Standalone Compose stack |
| `DESIGN.md` | Normative UI contract |

## Quick start (development)

```bash
# API
go run ./cmd/shipyard-server

# UI (yarn workspaces; pnpm-workspace.yaml also present)
yarn install
yarn workspace @shipyard/web dev

# Compose (Postgres + server)
docker compose -f deploy/compose/compose.yml up --build
```

## Normative docs

1. `DESIGN.md` — frontend / design system
2. `docs/architecture/` — backend architecture
3. `docs/handoff/` — Cursor handoff pack and implementation prompt

## License

Apache-2.0 (planned). Exact license file lands with the first public release prep.
