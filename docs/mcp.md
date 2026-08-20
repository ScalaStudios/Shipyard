# Shipyard for AI agents

Shipyard exposes its control plane both as a documented REST API and as a Model
Context Protocol (MCP) server, so AI agents and other tools can drive builds,
inspect runs, and manage runners.

## OpenAPI spec and docs page

The control plane serves its own OpenAPI 3.1 contract and a rendered reference:

- `GET /api/v1/openapi.json` — the spec as JSON
- `GET /api/v1/openapi.yaml` — the spec as YAML
- `GET /api/docs` — a human- and AI-readable reference page (Scalar)

All three are public and need no authentication. Point any OpenAPI-aware tool at
`/api/v1/openapi.json`.

## Authentication

API and MCP automation authenticate with a Bearer token:

```
Authorization: Bearer <api-token>
```

Use an **unscoped** API token. Tokens carrying registry scopes (`registry:read`,
`registry:write`) are rejected on the general API. Generate one in Settings, or
via `POST /api/v1/me/tokens` with no scopes.

## MCP server

`cmd/shipyard-mcp` is a stdio MCP server that wraps the REST API. Build it:

```bash
go build -o shipyard-mcp ./cmd/shipyard-mcp
```

It reads two environment variables:

- `SHIPYARD_URL` — base URL, e.g. `https://shipyard.scala.gg`
- `SHIPYARD_TOKEN` — an unscoped API token

Add it to an MCP client config:

```json
{
  "mcpServers": {
    "shipyard": {
      "command": "/path/to/shipyard-mcp",
      "env": { "SHIPYARD_URL": "https://shipyard.scala.gg", "SHIPYARD_TOKEN": "<unscoped-api-token>" }
    }
  }
}
```

## Tools

| Tool | Description |
|---|---|
| `shipyard_system_info` | Get instance information. |
| `shipyard_whoami` | Get the authenticated user. |
| `shipyard_list_orgs` | List organizations the token can access. |
| `shipyard_list_projects` | List projects in an organization (`orgID`). |
| `shipyard_list_pipelines` | List pipeline definitions in a project (`orgID`, `projectID`). |
| `shipyard_list_runs` | List runs in a project (`orgID`, `projectID`). |
| `shipyard_get_run` | Get a run and its jobs (`orgID`, `projectID`, `runID`). |
| `shipyard_start_run` | Start a run (`orgID`, `projectID`, `pipelineID`; optional `git_ref`, `git_sha`). |
| `shipyard_cancel_run` | Cancel a run (`orgID`, `projectID`, `runID`). |
| `shipyard_list_runners` | List runners visible to the token. |
