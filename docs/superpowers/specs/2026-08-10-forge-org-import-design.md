# Forge org import + app install design

**Status:** Draft for review (2026-08-10)  
**Goal:** Make it easy to bring a large forge organization (e.g. Scala on Forgejo with 60+ repos) into Shipyard as projects + SCM connections, with friendly multi-select import and first-class forge app / OAuth install paths.

## Product outcome

An operator can:

1. Create or pick a Shipyard organization.
2. Connect a forge account / install a forge app.
3. Browse a remote org’s repos with search and filters.
4. Select many repos (or all matching) and import them in one action.
5. End up with one Shipyard **project per forge repo**, each with an SCM connection and (where possible) an auto-registered webhook.

Default mapping: **1 forge repo → 1 Shipyard project** (slug from repo name).

## Non-goals (v1 program)

- Cloning code into Shipyard storage as part of import.
- Auto-authoring full pipeline YAML for every repo (optional stub later).
- GitLab group apps / Bitbucket in the first milestones (catalog stays open; adapters later).
- Replacing existing per-repo manual connection form (keep as advanced fallback).

## Architecture overview

```text
UI: Get started / Import from forge
        │
        ▼
shipyard-server
  scmimport  ── list orgs/repos, bulk create projects + connections
  scmauth    ── OAuth user tokens + GitHub App / Forgejo app credentials
  scm        ── existing connections, webhook ingest, bot comments
        │
        ▼
Forge APIs (Forgejo / Gitea / GitHub)
```

Reuse existing `scm` connection model. Add:

- **Forge credentials** (user OAuth token, GitHub App installation token source, Forgejo OAuth app tokens).
- **Import jobs** (async bulk import with progress + per-repo results).

## UX — Import wizard (all auth modes)

### Entry points

- Overview empty state → **Get started**
- Projects page → **Import from forge**
- Settings → Integrations → **Connect forge / Install app**

### Steps

1. **Destination** — select or create Shipyard org.
2. **Forge identity** — choose auth mode:
   - Personal access token (fastest path)
   - Sign in with forge OAuth (user)
   - Install GitHub App / Configure Forgejo OAuth application
3. **Remote org** — pick forge org (e.g. `Scala`).
4. **Repos** — searchable multi-select list tuned for 60–200 repos:
   - Debounced search (name / description)
   - Hide archived by default
   - Select all matching filter / clear
   - Sticky bar: `N selected` + Import
   - Already-imported repos marked Linked (skip by default)
   - Paginated or virtualized list (never render 60 heavy cards)
5. **Import** — progress UI; do not block the page on N sequential creates.
6. **Summary** — imported / skipped / failed with reasons; deep links into Projects.

## Auth modes

### A. PAT (Milestone 1 — ship first)

- Provider + base URL + token.
- Server calls forge REST with that token to list orgs/repos.
- Token stored encrypted like other secrets (`SHIPYARD_SECRETS_KEY`), scoped to the Shipyard org as a **forge credential**, not pasted into every connection row.
- Connections created during import reference the credential or copy a derived token material per existing connection schema (implementation chooses one; prefer credential reference if schema allows, else duplicate encrypted token into connections for compatibility with webhook bot).

### B. User OAuth (Milestone 2)

- OAuth apps for GitHub, Forgejo, Gitea, GitLab (API scopes for repo list + webhook write).
- Distinct from **login OIDC** (session identity). This is **forge API authorization** for the operator.
- Callback stores refresh/access tokens on a `forge_credentials` record.
- Same browse/import UI as PAT.

### C. GitHub App (Milestone 3)

- Configure GitHub App: App ID, private key, webhook secret, client id/secret (if user-to-server needed).
- Install URL → GitHub org install → callback with `installation_id`.
- Server mints installation tokens for list/import/webhook APIs.
- Prefer installation-level access for org-wide Scala-like orgs on GitHub.com / GHES.

### D. Forgejo / Gitea application (Milestone 3–4)

Forgejo does not have GitHub Apps 1:1. Do what’s good on Forgejo:

1. **OAuth2 application** (org or instance) for user-delegated API access — primary Forgejo path.
2. Document creating the OAuth app on `git.lunarlabs.dev` with redirect  
   `https://<shipyard>/api/v1/scm/oauth/forgejo/callback`.
3. Request scopes sufficient to: list orgs/repos, create repo hooks, post PR comments.
4. Optional later: instance admin / org deploy keys if Forgejo adds installable app primitives; do not block on them.

## Bulk import behavior

For each selected repo:

1. Create Shipyard project (`slug` = repo name, `name` = repo name / full name).
2. Create SCM connection (`provider`, `base_url`, `repo_owner`, `repo_name`, credential linkage).
3. If provider family supports it and token allows: **register webhook** to  
   `/api/v1/webhooks/{provider}?connection_id={id}` with secret.
4. Record result: `created` | `skipped_exists` | `failed`.

Idempotency: match existing connection by `(project or org scope) + provider + owner + name`; skip duplicates.

Concurrency: small worker pool (e.g. 4–8) with job progress `completed/total`.

## Data model (additive)

Suggested tables (names flexible):

- `forge_credentials` — org-scoped; kind `pat|oauth_user|github_app_install|forgejo_oauth`; encrypted secrets; base_url; provider.
- `forge_import_jobs` — status, totals, created_by, org_id.
- `forge_import_job_items` — repo identity, status, error, project_id, connection_id.
- Optional `github_app_installations` — installation_id, account login, credential_id.

Migrations only; no breaking change to `scm_connections`.

## API sketch

```text
POST /api/v1/orgs/{orgID}/forge/credentials
GET  /api/v1/orgs/{orgID}/forge/credentials
POST /api/v1/orgs/{orgID}/forge/credentials/{id}/remote-orgs
POST /api/v1/orgs/{orgID}/forge/credentials/{id}/remote-repos   # query: org, q, page, include_archived
POST /api/v1/orgs/{orgID}/forge/imports                         # { credential_id, remote_org, repos[] | select_all_matching }
GET  /api/v1/orgs/{orgID}/forge/imports/{jobID}

GET  /api/v1/scm/oauth/{provider}/start
GET  /api/v1/scm/oauth/{provider}/callback

GET  /api/v1/scm/github-app/install
GET  /api/v1/scm/github-app/callback
```

Exact shapes can tighten in the implementation plan.

## UI surfaces

- `GetStartedPage` or modal wizard from Overview / Projects.
- Integrations settings: credential list, “Install GitHub App”, “Connect Forgejo”, remaining manual connection form under **Advanced**.
- Import job toast / panel with live progress.

## Config / ops

Env examples (illustrative):

- `SHIPYARD_GITHUB_APP_ID`, `SHIPYARD_GITHUB_APP_PRIVATE_KEY_PATH`, `SHIPYARD_GITHUB_APP_CLIENT_ID`, `SHIPYARD_GITHUB_APP_CLIENT_SECRET`, `SHIPYARD_GITHUB_APP_WEBHOOK_SECRET`
- `SHIPYARD_FORGEJO_OAUTH_CLIENT_ID`, `SHIPYARD_FORGEJO_OAUTH_CLIENT_SECRET`, `SHIPYARD_FORGEJO_BASE_URL` (default `https://git.lunarlabs.dev`)
- Existing `SHIPYARD_SECRETS_KEY` required for storing tokens

Document setup in `docs/DEPLOY.md` (create Forgejo OAuth app, create GitHub App, permissions checklist).

## Milestone plan

| Milestone | Deliverable | Success for Scala-sized orgs |
|-----------|-------------|------------------------------|
| **M1** | PAT credential + remote org/repo browse + multi-select import + job progress + webhook auto-register (Forgejo/Gitea/GitHub) | Import N selected plugins without hand-filling 60 forms |
| **M2** | User OAuth forge connect (Forgejo + GitHub) feeding same wizard | No long-lived PAT paste for day-to-day |
| **M3** | GitHub App install path | Org-wide GitHub installs |
| **M4** | Polish Forgejo OAuth app docs + UX; harden idempotency, permissions errors, rate limits | Friendly failures when token can’t create hooks |

**Implementation order:** M1 → M2 → M3 → M4. Do not block M1 on Apps.

## Risks

- Forge API rate limits during bulk import → paced workers + clear progress.
- Missing webhook create permission → import still creates projects/connections; surface “register webhook manually” with copied URL.
- Slug collisions in Shipyard → suffix `-2` or fail item with reason.
- Secrets key unset → refuse to store credentials; show setup hint.

## Acceptance (program)

- Operator can import 20+ repos from a Forgejo org with search + multi-select without the UI locking up.
- Re-running import skips already linked repos.
- Forgejo webhook auto-registration works when the token has hook scope.
- GitHub App install path documented and functional for at least one test org (M3).
- Manual per-repo connection remains available.
