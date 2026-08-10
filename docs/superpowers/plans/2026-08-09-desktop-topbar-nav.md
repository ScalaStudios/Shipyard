# Desktop topbar navigation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the operator sidebar with a desktop-style topbar plus a mono path strip.

**Architecture:** Single `AppShell` owns chrome. Primary sticky bar = brand + section links + actions. Secondary sticky strip = path breadcrumb + org/project selects. Breadcrumb built from workspace context + active `NAV_ITEMS` + optional deeper route segments.

**Tech Stack:** React 19, React Router 7, CSS modules, Tabler icons, existing `@shipyard/ui` tokens.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-08-09-desktop-topbar-nav-design.md`
- No route URL scheme changes; keep existing React Router paths
- Keep Settings inner left subnav
- Harbor tokens / IBM Plex / glass sticky chrome / light+dark
- Icons + labels on comfortable widths; overflow Menu under ~960px

## File map

| File | Responsibility |
|---|---|
| `apps/web/src/shell/path.ts` | Pure helpers: section slug from pathname, breadcrumb segments |
| `apps/web/src/shell/AppShell.tsx` | New layout structure, overflow menu state |
| `apps/web/src/shell/AppShell.module.css` | Full-width shell, dual sticky bars, path chrome, overflow |

---

### Task 1: Path helpers

**Files:** create `apps/web/src/shell/path.ts`

- [ ] Add `sectionFromPath(pathname)` using `NAV_ITEMS` (longest prefix match; `/` = overview)
- [ ] Add `buildPathSegments({ orgSlug, projectSlug, pathname })` returning `{ label, to? }[]` for `/{org}/{project}/{section}` plus run id when `/pipelines/runs/:id`
- [ ] Commit

### Task 2: Rewrite AppShell layout

**Files:** `AppShell.tsx`, `AppShell.module.css`

- [ ] Remove sidebar; single-column shell
- [ ] Primary topbar: brand, horizontal nav (or Menu details/dropdown on narrow), actions
- [ ] Path strip: mono breadcrumb + org/project selects
- [ ] Active link underline/fill styles; sticky dual chrome
- [ ] `bun run build` in `apps/web`
- [ ] Commit

### Task 3: Smoke in browser

- [ ] Confirm Vite still serving; open UI and verify nav + path update across a few routes
