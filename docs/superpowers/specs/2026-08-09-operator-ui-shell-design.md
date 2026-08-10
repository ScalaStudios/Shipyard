# Operator UI shell — Phase UI-1

**Date:** 2026-08-09  
**Status:** Approved by direction (“just do what you gotta do”)  
**Contract:** Root `DESIGN.md`

## Goal

Replace the single Workspace screen with the normative operator shell and first-class pages that answer Jenkins-style CI questions and Artifactory-style artifact questions, plus an admin/settings surface.

## Navigation (fixed order)

1. Overview  
2. Projects  
3. Pipelines  
4. Artifacts  
5. Registry  
6. Releases  
7. Deployments  
8. Runners  
9. Cluster  
10. Settings  

## Patterns

- Application shell: topbar + left sidebar (`224px`) + page header + main work area  
- Primary pattern per area: **table/list**, **list + inspector** for run/job logs  
- Org/project context selector in the topbar  
- Medium-high density; tables first; no bento-card dashboard  

## First verticals

1. Shell + Overview (health, runners, recent runs, attention)  
2. Pipelines / run detail / logs / runners (CI console)  
3. Artifacts / packages / OCI / releases (registry console)  
4. Settings (members, tokens, secrets, system info)

## Out of scope for UI-1

- Command palette (`Cmd+K`)  
- Full DAG graph editor  
- Monaco YAML editor  
- Virtualized log follower beyond a solid scrollable log pane  
