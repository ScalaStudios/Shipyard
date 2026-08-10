# Shipyard agent instructions

These instructions bind AI coding agents working in this repository.

## Precedence

1. Existing repository code and this file
2. `DESIGN.md` (frontend / design contract)
3. `docs/architecture/Shipyard_Developer_Architecture_Spec.docx`
4. `docs/handoff/project-implementation-prompt.md`
5. Historical material in `docs/handoff/` (reference only)

## Product identity

Shipyard is one cohesive delivery platform:

```text
source → pipeline → job → artifact/package/image → release → deployment
```

It is not a Jenkins skin, Artifactory clone, or loosely coupled service collection.

## Engineering rules

- Prefer reuse over new abstractions.
- Do not invent architecture already defined by the specs.
- Build coherent vertical milestones (Phase 0 → Phase 9); do not scaffold the whole product unfinished.
- No generic purple SaaS dashboard UI. Follow `DESIGN.md`.
- Feature UI imports from `@shipyard/ui`, not raw third-party styled kits.
- Go control plane is a modular monolith (`shipyard-server`) with clear internal packages.
- PostgreSQL for transactional state; filesystem and S3-compatible blob storage for payloads.
- Standalone first; distributed runners and cluster/HA must remain possible without forking the codebase.

## Commits

When the user asks for commits:

- focused commits with `feat:`, `fix:`, `chore:`, or `refactor:` prefixes
- no AI attribution / `Co-Authored-By`
- never commit secrets, tokens, or local credential files

## Current milestone

Phase 0 — Foundation. Tracked in Forgejo issues under `Shipyard/shipyard`.
