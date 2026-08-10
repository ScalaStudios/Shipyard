# Desktop topbar navigation (operator shell)

**Status:** Approved for implementation (2026-08-09)  
**Scope:** `apps/web` AppShell + related CSS only  
**Replaces:** Left sidebar primary navigation

## Goal

Replace the sidebar with a desktop-style top chrome: primary section links across a sticky top bar, plus a secondary path strip that reads like a Linux location bar (`/{org}/{project}/{section}`).

## Layout

```text
┌──────────────────────────────────────────────────────────────┐
│ [mark] Shipyard   Overview · Pipelines · … · Settings   🔔 ☾ you │
├──────────────────────────────────────────────────────────────┤
│ /{org}/{project}/{section}[/{…}]              [org ▾] [proj ▾] │
├──────────────────────────────────────────────────────────────┤
│                         page content                           │
└──────────────────────────────────────────────────────────────┘
```

Two sticky rows above content; no left rail.

## Primary topbar

- Left: product mark + “Shipyard”
- Center/left-of-actions: horizontal `NAV_ITEMS` links
- Right: notification bell, theme toggle, account name + sign out
- Active section: subtle fill + underline (not a left-edge indicator)
- Links show **icon + label** on comfortable widths
- Narrow widths: collapse section links into a single Menu / overflow control; keep brand, path strip, and actions visible

## Path strip

- Mono breadcrumb: `/{org-slug}/{project-slug}/{section}` 
- Deeper routes append segments (e.g. run detail → `…/pipelines/{runId}`)
- Segments are links where a destination exists; current leaf is plain text
- Org/project `<select>`s move to the right side of this strip (same workspace context behavior as today)

## Out of scope

- Route URL scheme changes (keep existing React Router paths)
- Settings inner left subnav (keep for now)
- Command palette / `Cmd+K`
- Reworking individual page layouts beyond shell chrome

## Visual continuity

Keep existing harbor tokens: canvas gradients, glass blur on sticky chrome, route enter animation, IBM Plex, table-first pages.

## Files expected to change

- `apps/web/src/shell/AppShell.tsx`
- `apps/web/src/shell/AppShell.module.css`
- Possibly a small path helper next to `nav.ts` if breadcrumb logic needs isolation

## Acceptance

- Sidebar gone; content uses full width
- Every current nav destination reachable from the topbar (or overflow menu)
- Path strip updates with route + org/project context
- Org/project switching still works
- Usable at ~960px and below via overflow
- Light/dark themes still work
