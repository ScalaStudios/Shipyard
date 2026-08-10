<p align="center">
  <img src="docs/brand/shipyard-mark.svg" alt="Shipyard" width="96" height="96" />
</p>

<h1 align="center">Shipyard</h1>

<p align="center">
  <strong>One control plane for software delivery.</strong><br />
  Pipelines, distributed runners, artifacts, npm &amp; Maven packages, an OCI registry, releases and deployments — in a single self-hosted product.
</p>

<p align="center">
  <a href="LICENSE"><img alt="License: Apache 2.0" src="https://img.shields.io/badge/license-Apache%202.0-blue" /></a>
  <img alt="Go 1.25" src="https://img.shields.io/badge/go-1.25-00ADD8?logo=go&logoColor=white" />
  <img alt="PostgreSQL 16" src="https://img.shields.io/badge/postgres-16-4169E1?logo=postgresql&logoColor=white" />
  <img alt="React + TypeScript" src="https://img.shields.io/badge/ui-react%20%2B%20typescript-61DAFB?logo=react&logoColor=black" />
  <img alt="Self-hosted" src="https://img.shields.io/badge/self--hosted-yes-success" />
  <img alt="Status: alpha" src="https://img.shields.io/badge/status-alpha-orange" />
</p>

```text
source → pipeline → job → artifact / package / image → release → deployment
```

Most teams glue this path together from four or five separate tools. Shipyard is one binary, one database and one UI that covers the whole line — self-hosted, with no per-seat pricing and no SaaS dependency.

> **Status: alpha.** It runs real pipelines against real forges today, and the schema and HTTP API are still moving. Pin a commit if you deploy it.

---

## Features

| Area | What you get |
|---|---|
| **Pipelines** | YAML pipelines, dependency graph, jobs, steps, live logs, cancellation |
| **Runners** | Distributed runners with labels, heartbeats, leases and one-command install |
| **Forge import** | Bring a whole org in at once — PAT, user OAuth, or a GitHub App install |
| **Artifacts** | Content-addressed storage on filesystem or S3 |
| **Packages** | npm and Maven repositories, with `maven-metadata.xml` generation |
| **Registry** | OCI `/v2` distribution with token auth — `docker push` works |
| **Delivery** | Releases, environments and deployments |
| **Auth** | Local accounts, sessions, API tokens, and OIDC sign-in (GitHub, GitLab, Forgejo, Gitea, Entra, Discord) |
| **Integrations** | Forge webhooks, PR/MR bot comments, commit statuses, Discord alerts |
| **Ops** | Instance settings in the database, encrypted credentials, cluster heartbeat, setup checklist |

Credentials — forge tokens, OAuth client secrets, GitHub App keys, webhook secrets — are encrypted at rest with AES-GCM. GitHub App installation tokens are minted on demand and never stored.

## Quick start

Requires Docker (rootless is fine), [Bun](https://bun.sh), and Go 1.25 for the runner.

```bash
# API + Postgres
docker compose -f deploy/compose/compose.yml up --build -d

# Operator UI
bun install
bun run dev
```

Open <http://localhost:5173> and create the first account — it becomes the instance admin. The **Get started** checklist walks you through the rest.

To encrypt credentials (required before you can store any), set a key before starting the server:

```bash
echo "SHIPYARD_SECRETS_KEY=$(head -c 32 /dev/urandom | base64)" >> deploy/compose/.env
```

## Deploy on a domain

```bash
cd deploy/compose
cp .env.prod.example .env          # domain, ACME email, DB password, secrets key
docker compose -f compose.prod.yml up --build -d
```

That brings up Postgres, the control plane, and Caddy serving the built UI with automatic HTTPS — API and UI share one origin. Point your DNS at the host and open port 80 and 443.

Full guide, including nginx and forge webhook setup: **[docs/DEPLOY.md](docs/DEPLOY.md)**.

## Repository layout

| Path | Purpose |
|---|---|
| `cmd/shipyard-server` | Control plane |
| `cmd/shipyard-runner` | Job executor |
| `cmd/shipyard` | CLI |
| `internal/` | Pipelines, runners, packages, registry, scm, secrets, settings |
| `apps/web` | Operator UI (React + TypeScript) |
| `packages/ui` | `@shipyard/ui` design system |
| `deploy/compose` | Dev and production stacks |
| `deploy/caddy`, `deploy/nginx` | Reverse proxy configs |
| `migrations/` | SQL migrations, applied on boot |

## Development

```bash
go test ./...                        # Go tests
bun run --filter @shipyard/web build # type-check and build the UI
./scripts/smoke.sh                   # API smoke test, server must be up
```

Design and contribution rules live in [`DESIGN.md`](DESIGN.md) (normative UI contract) and [`AGENTS.md`](AGENTS.md).

## Contributing

Issues and pull requests are welcome. Please keep changes focused, include a test for non-trivial logic, and run `go test ./...` plus the UI build before opening a PR.

## Security

Do not open a public issue for a vulnerability. Report it privately to the maintainers, and give us a reasonable window to ship a fix before disclosure.

## License

[Apache License 2.0](LICENSE).
