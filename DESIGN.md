# Shipyard UI/UX Design System & Implementation Contract

**Status:** Normative  
**Research snapshot:** 2026-08-09  
**Applies to:** Shipyard Web UI, reusable `@shipyard/ui` components, Storybook, generated screenshots, and AI-assisted frontend work.

> This file is not inspiration. It is the implementation contract. Human contributors and AI coding agents MUST follow it unless a maintainer explicitly approves an exception.

---

## 0. Non-negotiable rules for humans and AI agents

1. **Do not make Shipyard look like a generated SaaS dashboard.** Do not default to giant rounded cards, purple/blue gradients, glass panels, excessive pills, large empty whitespace, glowing CTAs, sparkle icons, or a generic shadcn demo appearance.
2. **Shipyard is an engineering workspace.** Optimize for recognition of state, diagnosis, comparison, and safe action.
3. **Use Shipyard-owned components.** Product code imports reusable UI from `@shipyard/ui`. Do not import a pre-styled component library directly into feature screens.
4. **Accessible behavior comes from React Aria Components.** Styling and visual identity remain Shipyard-owned. [R14]
5. **Use semantic design tokens.** Do not hard-code colors, radii, shadows, or spacing in feature components unless the design-system package does not yet provide a required token.
6. **Tables are first-class.** Do not replace useful tabular data with a grid of cards. Use TanStack Table for complex data tables. [R15]
7. **Graphs are optional views, not the only truth.** Pipeline DAG and cluster topology screens MUST also provide list/table views. Argo CD's multiple detail views are a useful precedent. [R12]
8. **Dark, light and system theme modes are required.** Do not implement dark mode through one-off selectors; semantic tokens drive themes. [R2][R9]
9. **Keyboard support is required.** Every primary workflow must be operable without a mouse.
10. **Shipyard uses IBM Plex Sans and IBM Plex Mono.** Do not substitute Inter unless the font cannot load. IBM Plex is open source and designed for UI use. [R17]
11. **Product icons use Tabler Icons.** Brand/vendor marks use Simple Icons or official vendor SVGs. Do not mix Lucide, Heroicons, Font Awesome and Tabler on the same surface. [R16]
12. **Ordinary CI logs are not a fake terminal.** Use a purpose-built virtualized log viewer. Use xterm.js only for a genuinely interactive exec/shell session.
13. **One obvious primary action per action group.** Secondary actions should be visually quiet.
14. **Status is never communicated by color alone.** Pair semantic color with icon and text.
15. **No feature is UI-complete without loading, empty, error, permission-denied, disconnected and partial-data states.**

---

## 1. Design objective

Shipyard combines CI/CD, artifact/package hosting, OCI registry functions, runners and cluster operations. Its UI should feel closer to a high-quality developer workstation than a marketing website.

The visual language is **industrial precision with subtle harbor identity**:

- graphite and neutral surfaces;
- Shipyard blue for selection and primary actions;
- restrained dock orange for rare release/attention accents;
- compact geometry;
- explicit borders and separators;
- dense information with strong hierarchy;
- stable navigation and predictable action locations;
- technical typography and tabular numerals;
- no decorative nautical gimmicks.

The product should still be friendly to new operators: density is not an excuse for clutter. The research shows that mature FOSS products do not converge on one fashionable framework; instead, projects such as GitLab and Grafana maintain their own design systems, Nextcloud standardizes layout patterns, and PatternFly treats dense enterprise data workflows as a design problem. [R1][R4][R7][R8]

---

## 2. Research summary and what Shipyard should borrow

### GitLab / Pajamas

Borrow:
- design tokens as the shared contract between design and code;
- versioned component/pattern libraries;
- dark-mode review as a normal part of design QA;
- clear separation between foundations, components, patterns, icons and visualization. [R1][R2]

Do not copy:
- GitLab-specific brand styling;
- every page's complexity or navigation density.

### Grafana / Saga

Borrow:
- treat the design system as a maintained software product;
- document components with limitations and intended use;
- bake accessibility into components rather than fixing it after features ship;
- expose technical data without trying to turn every screen into a marketing layout. [R4][R5]

Do not copy:
- the recognizable Grafana dashboard visual skin.

### Nextcloud

Borrow:
- predictable left navigation;
- content + sidebar/detail-inspector layouts;
- list + content patterns;
- theme-aware colors and restrained use of the primary brand color. [R6][R7]

Do not copy:
- consumer-file-manager spacing where Shipyard needs a denser operator view.

### PatternFly

Borrow:
- 14px default technical body scale;
- tabular numerals for changing numeric values;
- strong table patterns;
- semantic light/dark tokens. [R8][R9]

Do not copy:
- Red Hat's visual identity or the full PatternFly component suite.

### Harbor

Harbor's current portal identifies itself as an Angular + Clarity UI. The important lesson is not the framework; it is that artifact/registry administration benefits from straightforward project/repository/tag hierarchy. [R10]

### Gitea

Gitea's maintainers have discussed moving away from a heavy styled dependency toward flexible low-level/headless UI pieces. That supports Shipyard's decision to own its visual layer while using lower-level accessibility primitives. [R11]

### Argo CD

Argo CD allows operators to choose among tree, pod, network and list application-detail views. Shipyard should follow the same principle: one visualization should not be forced on every operator or every scale. [R12]

### Portainer

Portainer's current repository shows an incremental move toward modern React tooling while still carrying older framework code. Shipyard should avoid that migration burden by defining a stable UI package and feature boundaries early. [R13]

### Community preference signals

Public developer discussions are **qualitative, not a survey**. They do consistently surface three useful signals:

- power users often value higher information density, provided hierarchy and readability remain clear; [Q1][Q2]
- keyboard-driven access reduces interaction overhead for technical workflows;
- mobile tables work better when the UI prioritizes the user's task, preserves identity columns, and uses expansion/column selection instead of blindly squeezing desktop tables. [Q3]

Shipyard therefore defaults to **medium-high desktop density**, with Compact and Comfortable table density options.

---

## 3. Frontend architecture

### Required stack

| Concern | Choice | Rule |
|---|---|---|
| Framework | React + TypeScript + Vite | Keep feature code framework-simple. |
| Accessible behavior | `react-aria-components` | Wrap inside `@shipyard/ui`; never accept its example styling as Shipyard styling. [R14] |
| Styling | CSS Modules + CSS custom properties | No CSS-in-JS requirement and no stock component-library theme. |
| Server state | `@tanstack/react-query` | Async/server state only. |
| Data tables | `@tanstack/react-table` | Headless logic; Shipyard owns HTML, density and styling. [R15] |
| Virtualization | `@tanstack/react-virtual` | Large tables, logs, artifact lists, runner fleets. |
| DAG/topology | `@xyflow/react` | Custom Shipyard nodes; list/table alternative required. |
| YAML/code | `monaco-editor` | `shipyard.yml`, diffs, policy/config editors. |
| Log viewer | custom + virtualization + ANSI parser | Search, timestamps, wrap, pause/follow, copy selection. |
| Interactive shell | `@xterm/xterm` | Only real shell/exec sessions. |
| Operational charts | `uPlot` | Small fast time-series; no generic chart-card factory. |
| Icons | `@tabler/icons-react` | Only general product icon family. [R16] |
| Brand icons | Simple Icons / official SVG | Vendors only. |
| Component docs | Storybook | Every reusable state documented. |
| E2E | Playwright | Critical workflows + keyboard paths. |
| Accessibility testing | axe-core + manual keyboard/SR checks | Automated checks are a floor, not the finish line. |

### Explicitly not canonical

- shadcn/ui as the visual system;
- Material UI, Ant Design, Chakra, Mantine or another pre-styled suite as the visual owner;
- Bootstrap-style page composition;
- Tailwind component recipes copied directly from templates.

A contributor MAY use a utility framework only inside `@shipyard/ui` if maintainers adopt it later. Feature screens must still consume semantic Shipyard components/tokens.

### Package boundaries

```text
apps/
  web/
    src/
      app/
      features/
      routes/
      shell/
packages/
  ui/
    src/
      components/
      patterns/
      tokens/
      icons/
      typography/
      styles/
  pipeline-graph/
  log-viewer/
  config-editor/
  api-client/
  domain/
```

Feature code must not duplicate a component already owned by `packages/ui`.

---

## 4. Design-token system

### Token source of truth

Use a generated token pipeline:

```text
packages/ui/src/tokens/
  source.json          # canonical values + semantic mapping
  generated.css        # CSS custom properties
  generated.ts         # typed TS token names/values
  README.md             # token intent
```

A small repository script may generate CSS/TS from `source.json`. Do not maintain duplicate hand-edited values.

### Naming

Use semantic names:

```css
--sy-color-bg-canvas
--sy-color-bg-surface
--sy-color-text-primary
--sy-color-text-muted
--sy-color-border-default
--sy-color-action-primary
--sy-color-status-success
--sy-space-4
--sy-radius-panel
--sy-control-height-default
```

Do not write feature CSS using palette-only names such as `--blue-500` when the meaning is actually `action-primary`.

---

## 5. Color system

### Dark theme

| Token | Value | Usage |
|---|---:|---|
| `bg.canvas` | `#0B0D10` | app background |
| `bg.surface` | `#111419` | navigation, main panes |
| `bg.subtle` | `#15191F` | nested/alternate surface |
| `bg.raised` | `#191E25` | popovers/dialogs only |
| `border.default` | `#2A3038` | separators |
| `border.strong` | `#3A424D` | strong boundary |
| `text.primary` | `#E7EAF0` | primary text |
| `text.muted` | `#9AA4B2` | secondary metadata |
| `text.faint` | `#6F7A87` | low-priority metadata |
| `action.primary` | `#6E97FF` | selection/primary action |
| `action.primaryHover` | `#89AAFF` | hover |
| `accent.dock` | `#E2A45B` | rare Shipyard accent |
| `status.success` | `#45B97C` | success/healthy |
| `status.warning` | `#D8A13B` | warning/degraded |
| `status.danger` | `#E26666` | fail/destructive |
| `status.info` | `#69A7E8` | info |
| `focus.ring` | `#9AB8FF` | keyboard focus |

### Light theme

| Token | Value | Usage |
|---|---:|---|
| `bg.canvas` | `#F5F7F9` | app background |
| `bg.surface` | `#FFFFFF` | main pane/navigation |
| `bg.subtle` | `#F8FAFC` | nested/alternate surface |
| `border.default` | `#D8DEE6` | separators |
| `border.strong` | `#B7C0CC` | strong boundary |
| `text.primary` | `#1F2328` | primary text |
| `text.muted` | `#59636E` | secondary text |
| `text.faint` | `#7A8591` | low priority |
| `action.primary` | `#315EFB` | primary action |
| `action.primaryHover` | `#244CD8` | hover |
| `accent.dock` | `#B86519` | rare accent, not default body text |
| `status.success` | `#177A49` | success/healthy |
| `status.warning` | `#A05E00` | warning/degraded |
| `status.danger` | `#B92C2C` | fail/destructive |
| `status.info` | `#256FAF` | info |
| `focus.ring` | `#1F5EFF` | focus |

### Color rules

- No gradients in normal product chrome.
- Never use status red/green as a decorative accent.
- Primary blue means **selection or primary action**, not “anything important.”
- Dock orange is limited to rare release/attention moments and brand details.
- In tables, prefer a subtle tinted row/background + icon + label instead of fully saturated status fills.
- Never use colored body text when the contrast is insufficient; use primary text plus a colored icon/border instead.

---

## 6. Typography

### Fonts

- **UI:** IBM Plex Sans
- **Code/technical:** IBM Plex Mono
- **Fallback:** system sans / system monospace

IBM Plex provides Sans and Mono families under an open font license and is designed to work well in user interfaces. [R17]

### Scale

| Token | Size / line | Weight | Use |
|---|---|---:|---|
| Page title | 24 / 32px | 600 | one per page |
| Section | 18 / 26px | 600 | major section |
| Panel title | 15 / 22px | 600 | panel/table heading |
| Body | 14 / 20px | 400 | default |
| Body strong | 14 / 20px | 500 | names/important values |
| Small | 13 / 18px | 400 | dense tables/metadata |
| Micro | 12 / 16px | 500 | badges/timestamps |
| Code | 13 / 20px | 400 | logs/YAML/commands |
| Code small | 12 / 18px | 400 | digests/coordinates |

PatternFly's use of 14px default body text and tabular numerals is a useful reference for dense technical interfaces. [R8]

### Rules

- Sentence case.
- Avoid all caps except short protocol/architecture labels (`OCI`, `AMD64`, `ARM64`).
- Use `font-variant-numeric: tabular-nums` for build numbers, duration, storage, percentages, queue counts and changing metrics.
- Do not use monospace as decoration; use it for machine-readable content.
- Page headings are left aligned.
- Avoid huge 32-48px headings inside the authenticated product UI.

---

## 7. Geometry, spacing and density

### 4px base grid

`0, 4, 8, 12, 16, 20, 24, 32, 40, 48, 64`

### Radius

- controls: `6px`
- panels/popovers: `8px`
- large dialogs: max `10-12px`
- pills: only when the semantic shape is a pill (tag/status/chip)

### Control heights

- compact desktop: `28px`
- default desktop: `32px`
- prominent form action: `36px`
- touch-oriented effective target: aim near `44px` where practical while meeting WCAG minimums. [R18]

### Table row density

- Compact: `30px`
- Comfortable: `36px` (default)

Users may choose Compact or Comfortable globally. Do not invent a third density per page.

### Elevation

**Borders before shadows.** Shadows are reserved for overlays, floating menus and dialogs. Normal cards/panels use a 1px border and spacing.

---

## 8. Iconography

### Canonical product icons

Use direct imports from `@tabler/icons-react`.

| Meaning | Icon |
|---|---|
| Dashboard | `IconLayoutDashboard` |
| Projects | `IconFolderCode` |
| Pipelines | `IconGitBranch` |
| Job/run | `IconPlayerPlay` |
| Artifacts | `IconPackage` |
| Registry | `IconContainer` |
| Releases | `IconRocket` |
| Deployments | `IconCloudUpload` |
| Runners | `IconServer2` |
| Cluster | `IconTopologyStar3` |
| Logs | `IconTerminal2` |
| Settings | `IconSettings` |
| Secrets | `IconKey` |
| Security | `IconShieldCheck` |
| Success | `IconCircleCheck` |
| Failure | `IconCircleX` |
| Running | `IconLoader2` |
| Queued | `IconClock` |
| Warning | `IconAlertTriangle` |
| Retry | `IconRefresh` |
| Stop | `IconSquare` |
| Copy | `IconCopy` |
| Download | `IconDownload` |
| Upload | `IconUpload` |
| Search | `IconSearch` |
| Command palette | `IconCommand` |

Tabler provides React SVG icons under an MIT license. [R16]

### Rules

- navigation: 18px
- normal control: 16px
- stroke: library default unless design-system wrapper changes it globally
- icon-only buttons MUST have an accessible label and tooltip
- do not use emoji as product icons
- do not use sparkle/wand/brain icons for generic “smart” functions
- vendor brands must not be recolored in ways that violate their brand rules

---

## 9. Application shell

### Desktop shell

```text
+--------------------------------------------------------------+
| Topbar: project/context | global search | status | account   |
+------------+-------------------------------------------------+
| Sidebar    | Page header: title | context | primary action   |
|            +-------------------------------------------------+
| Overview   | Toolbar / filters / tabs                         |
| Projects   +-------------------------------------------------+
| Pipelines  | Main work area                                  |
| Artifacts  |                                                  |
| Registry   |                         [optional inspector]      |
| Releases   |                                                  |
| Deploy     |                                                  |
| Runners    |                                                  |
| Cluster    |                                                  |
+------------+-------------------------------------------------+
```

Dimensions:
- topbar: `48px`
- sidebar expanded: `224px`
- sidebar collapsed: `56px`
- desktop page gutter: `20-24px`

### Navigation order

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

Do not reorder navigation based on usage analytics or permissions. Hide unavailable sections when appropriate, but preserve relative order.

### Command palette

`Cmd/Ctrl+K` opens global navigation/action search.

Initial commands:
- open project
- open pipeline/run
- open artifact by digest/name
- open runner
- open environment
- trigger pipeline (when permitted)
- create release (when permitted)
- theme switch

Dangerous actions do not execute immediately from command search; they open the relevant confirmation flow.

---

## 10. Component contract

### Button

- variants: primary, secondary, ghost, destructive, link, icon-only
- sizes: 28 / 32 / 36px
- radius: 6px
- icons: 16px
- one high-emphasis primary action per local action group
- destructive buttons are not visually primary until the user enters a destructive confirmation context

### Input

- default height: 32px
- persistent visible label
- helper/error below field
- code/path/token inputs may use IBM Plex Mono
- do not use floating labels

### Select / ComboBox

Use React Aria behavior. [R14]

- typeahead required
- searchable when options > ~8 or when options are remote
- clear selected/value state
- multi-select only where the domain truly supports multiple choices

### Badge / chip

Allowed for:
- status
- environment
- role
- tag
- runner label

Not allowed for every metadata value. Plain text is often better.

### Dialog

- typical max width: 560px
- use for create/edit/confirm
- destructive dialog names the target and states the impact
- typing the object name is reserved for high-impact irreversible operations, not every delete

### Inspector drawer

Use a right-side inspector for selected pipeline nodes, artifacts, runners, deployments and cluster members when keeping list/DAG context is useful.

### Toast

- transient confirmation and low-risk information
- a critical failure must also remain visible in the relevant page/state
- never make a toast the only record of a failed operation

---

## 11. Tables and lists

Tables are a core Shipyard interaction surface.

### Use a table when

- multiple objects share the same properties;
- users compare state across rows;
- sorting/filtering/bulk action matters;
- identity and numeric/technical fields need scanability.

### Standard table anatomy

```text
[ Search ........ ] [Status v] [Owner v] [Columns] [Density]   [Action]
-----------------------------------------------------------------------
Name              Status     Duration      Commit       Started      ...
api-build #1842   Failed     04:31         a82c10f      2m ago        ...
worker #1841      Success    03:12         118faca      9m ago        ...
```

### Required capabilities for complex tables

- semantic `<table>` markup whenever practical;
- stable identity/name column;
- sortable headers;
- filters displayed above the table, not hidden in a mystery icon;
- column chooser;
- density selector;
- sticky header for long lists;
- optional sticky identity column;
- row selection only if a bulk action exists;
- right-align numeric values;
- tabular numerals;
- virtualization for very large row sets;
- preserve query/filter state in the URL when useful.

TanStack Table is headless and allows Shipyard to keep full control over markup and styling. [R15]

### Mobile table strategy

Do not blindly transform every row into a huge card.

Priority order:
1. preserve identity and most important status/action;
2. hide lower-priority columns;
3. allow row expansion;
4. provide column selection for expert users;
5. use horizontal scroll only when comparison across columns is the task.

This is consistent with qualitative feedback from developers discussing mobile data tables. [Q3]

---

## 12. Pipeline experience

### Pipeline run desktop layout

```text
+-------------------------------------------------------------+
| Build #1842  FAILED    commit a82c10f   branch production   |
| [Retry failed] [Cancel] [More]                              |
+-------------------------------------------------------------+
| [Graph] [List] [Timeline]                                   |
+--------------------------------------+----------------------+
|                                      | Inspector            |
|  checkout -> test -> build -> image  | build                |
|                 \-> security         | Failed in 04:31      |
|                                      | runner linux-04      |
|                                      | artifact server.jar  |
+--------------------------------------+----------------------+
| Live log panel (resizable)                                   |
+-------------------------------------------------------------+
```

### Pipeline graph rules

- custom Shipyard node design; do not use React Flow example styling;
- node geometry should remain compact;
- state uses icon + label + semantic color;
- edge animation only while a transition is actively running and only when reduced motion is not requested;
- selected node opens the inspector without navigating away;
- zoom controls are quiet and predictable;
- graph fit should not make text unreadably small;
- **List view is mandatory** and remembers the user's last choice.

Argo CD's support for multiple operational views is an important precedent. [R12]

### Job state design

Every job row/node shows:
- job name
- state
- elapsed/final duration
- runner or executor
- retry count when >0
- optional artifact indicator

Failure details prioritize:
1. failed step;
2. concise failure reason;
3. relevant log location;
4. retry action if safe;
5. runner/executor context.

---

## 13. Log viewer

### Ordinary CI logs

Build a dedicated log viewer, not a terminal emulator skin.

Features:
- streaming append without destroying text selection;
- Follow toggle;
- Pause stream;
- search;
- regex search optional later;
- timestamp toggle;
- line numbers optional;
- wrap toggle;
- ANSI colors with Shipyard-safe palette mapping;
- copy selected;
- download full log;
- jump to first error/warning when structured markers exist;
- virtualized rendering;
- preserve scroll position when the user scrolls away from the bottom.

### Interactive shell

Only when a user explicitly starts a live shell/exec session should Shipyard use xterm.js. Separate this visually and semantically from immutable CI logs.

---

## 14. Artifact and package UX

### Artifact browser

Default view is a structured list/table, not a tile gallery.

Columns:
- name/path
- type
- size
- checksum/digest
- producing pipeline
- version/tag
- created
- retention state

Selecting an artifact opens an inspector with:
- canonical identifier;
- SHA-256;
- MIME/package type;
- source commit;
- build/run;
- runner/executor;
- SBOM/provenance links when available;
- download/copy command;
- retention and delete policy.

The primary interaction model is:
**identify -> verify -> copy/use -> trace provenance**.

### Package page

Show package-manager-native instructions as copyable commands.

Example:

```text
Maven
com.example:shipyard-agent:1.4.2

Repository
https://shipyard.example.com/maven/releases

[Copy Gradle] [Copy Maven]
```

Do not hide install coordinates behind a secondary modal.

---

## 15. OCI registry UX

Repository page:
- repository name
- pull command
- tag count
- storage usage
- latest push
- policy/security summary

Tag/manifest table:
- tag
- digest
- media type
- platforms
- size
- pushed
- signer/provenance status

Manifest inspector:
- digest first;
- copyable pull-by-digest command;
- architecture/OS variants;
- layer list;
- config metadata;
- referrers/attestations/SBOM;
- source pipeline and commit.

Avoid Docker-like decorative whale/container imagery in normal operations. Domain identity should come from terminology and information structure.

---

## 16. Releases and deployments

### Release page

A release connects:

```text
source commit -> pipeline -> artifacts/images -> release -> environments
```

Page sections:
- release identity/version
- source commit/tag
- artifacts/images included
- provenance/security summary
- changelog/notes
- deployment timeline

### Deployment page

Always display the environment context near the action area:

```text
PRODUCTION
us-east-1 / payments
currently: 1.8.2
candidate: 1.9.0
```

Production actions must not be visually indistinguishable from dev/staging actions.

Rollback:
- specify target release/digest;
- show what changes;
- require confirmation;
- show resulting deployment event in the timeline.

---

## 17. Runners and cluster operations

### Runner fleet

Use a table with:
- name
- online/draining/offline
- executor
- labels
- version
- architecture
- active/capacity
- last heartbeat
- current job

Bulk actions:
- drain
- enable/disable scheduling
- assign/remove labels

### Cluster page

Health comes before topology.

Top section:
- control-plane quorum/availability
- scheduler leader/lease state
- PostgreSQL health
- object storage health
- queue/message layer health when enabled
- connected runners

Then:
- member table
- dependency health
- topology view

Topology is secondary; operators must be able to understand health from text/table state alone.

---

## 18. Responsive design

### Desktop first, not desktop only

Shipyard is a technical operator product. Full creation/configuration workflows target desktop/tablet widths. Mobile must still support monitoring, approvals, incident checks and common safe actions.

Breakpoints are content-driven, not device-brand-driven.

At narrow widths:
- sidebar becomes a drawer;
- inspector becomes a full-width detail route/sheet;
- table preserves identity + status + one key metric;
- lower-priority metadata moves into row expansion;
- graphs default to list view when the graph cannot remain readable;
- logs keep search/follow/wrap but collapse secondary controls.

Do not create a completely different mobile information architecture.

---

## 19. Motion

Motion is functional only.

Allowed:
- 100-160ms hover/focus/selection transition;
- 160-220ms menu/dialog/drawer entrance;
- small progress indication;
- graph edge activity while currently running.

Not allowed:
- ambient gradients;
- glowing borders;
- infinite decorative pulses;
- springy page transitions;
- animated counters that delay reading;
- parallax.

Respect `prefers-reduced-motion` and remove nonessential movement.

---

## 20. Accessibility

Target: **WCAG 2.2 AA**.

Required:
- semantic headings/landmarks;
- every form control labeled;
- visible focus ring;
- no keyboard traps;
- meaningful focus return after dialogs;
- status icon + text + color;
- sufficient contrast in both themes;
- reduced-motion support;
- graph alternative;
- accessible table headers/captions where appropriate;
- no critical information only in hover/tooltip;
- live regions used sparingly.

React Aria provides style-free accessible interaction primitives, focus management and internationalization support, making it appropriate for Shipyard's owned design system. [R14] Grafana's accessibility guidance similarly warns against indiscriminate live-region use. [R5]

### Definition of done for a component

A reusable component is not done until Storybook includes:
- default
- hover/focus/pressed
- disabled
- loading if relevant
- invalid/error if relevant
- dark theme
- light theme
- long text
- keyboard interaction
- accessible name/label behavior

---

## 21. “Do not make it AI-looking” guardrails

This section is deliberately explicit because generic generated frontend code tends to converge on a recognizable set of visual shortcuts.

### Never default to

- purple -> blue -> cyan gradients;
- glassmorphism;
- translucent floating nav;
- 16-24px radii on every component;
- “bento” dashboard made entirely from cards;
- a giant `Welcome back` hero in an operator console;
- four giant KPI cards as the entire dashboard;
- a pill around every status, label, branch, timestamp and value;
- oversized icons in colored circles;
- sparkles/brain/wand icons for automation;
- fake terminal chrome for plain logs;
- glowing primary buttons;
- centered marketing copy inside authenticated pages;
- excessive shadows;
- placeholder charts that exist only to fill space;
- random gradients on avatar/project initials;
- stock shadcn demo spacing and component composition.

### Use instead

- tables and lists for comparable objects;
- split panes for inspect-without-losing-context workflows;
- subtle section backgrounds;
- borders/separators;
- one strong primary action;
- compact metadata;
- text labels and domain icons;
- provenance paths;
- explicit operational state;
- meaningful empty states with a single next step.

### Review question

Before merging a new screen, ask:

> If the Shipyard logo and product name disappeared, would this screen still look obviously designed for CI/CD, artifacts, registries, runners or cluster operations — or would it look like a generic AI-generated admin dashboard?

If the answer is “generic dashboard,” revise it.

---

## 22. Dashboard blueprint

The dashboard should answer:
1. Is Shipyard healthy?
2. What recently failed or changed?
3. Is capacity constrained?
4. What action needs attention?

Recommended composition:

```text
Shipyard
System healthy | 24 runners | 3 jobs queued | 68% artifact storage
------------------------------------------------------------------
Needs attention
! Pipeline #1842 failed   production/api        4m ago    [Open]
! Runner linux-07 offline cluster-east          9m ago    [Open]
------------------------------------------------------------------
Recent pipelines                         Recent releases
#1842 failed  api     a82c10f  4m ago    1.9.0  production  31m ago
#1841 success web     118faca  9m ago    1.8.9  staging     2h ago
------------------------------------------------------------------
Capacity / storage mini time series (only if it drives action)
```

Avoid making each line above a separate oversized rounded card.

---

## 23. AI coding-agent procedure

When an AI agent is asked to build or modify Shipyard UI, it MUST execute this sequence:

### Step 1 - classify the screen

Choose one primary pattern:
- table/list;
- list + inspector;
- graph + inspector;
- settings form;
- editor;
- operational dashboard;
- timeline/detail;
- terminal/log viewer.

Do not invent a novel page pattern unless none fits.

### Step 2 - identify the operator question

Write one sentence internally:

`The user opens this screen to answer: ______.`

Every major UI element must help answer that question or trigger the next safe action.

### Step 3 - reuse components

Search `@shipyard/ui` and existing patterns before creating new UI.

Do not:
- clone a button/input/table style inside a feature folder;
- import a new visual component library to solve one screen;
- add a second icon family.

### Step 4 - implement all states

At minimum:
- loading;
- loaded;
- empty;
- error;
- permission denied when applicable;
- stale/disconnected when applicable.

### Step 5 - implement accessibility

Verify:
- keyboard path;
- focus order;
- visible focus;
- labels;
- screen-reader names;
- status not color-only;
- reduced motion.

### Step 6 - test density and long content

Use fixture data with:
- 100+ rows;
- very long project names;
- long artifact paths;
- full SHA-256 digests;
- several status types;
- zero results;
- partial data.

### Step 7 - run the anti-generic review

Reject the implementation if it introduced:
- gradients;
- excessive rounded cards;
- random pill metadata;
- arbitrary shadows;
- generic hero/marketing sections;
- stock component-demo layout.

### Step 8 - Storybook and tests

Add/modify:
- Storybook story for reusable UI;
- Playwright path for critical workflow;
- accessibility check;
- visual regression snapshot where maintained.

---

## 24. Example implementation conventions

### Semantic token CSS

```css
:root {
  --sy-color-bg-canvas: #f5f7f9;
  --sy-color-bg-surface: #ffffff;
  --sy-color-text-primary: #1f2328;
  --sy-color-text-muted: #59636e;
  --sy-color-border-default: #d8dee6;
  --sy-color-action-primary: #315efb;
  --sy-radius-control: 6px;
  --sy-radius-panel: 8px;
  --sy-control-height-default: 32px;
}

[data-theme="dark"] {
  --sy-color-bg-canvas: #0b0d10;
  --sy-color-bg-surface: #111419;
  --sy-color-text-primary: #e7eaf0;
  --sy-color-text-muted: #9aa4b2;
  --sy-color-border-default: #2a3038;
  --sy-color-action-primary: #6e97ff;
}
```

### Component imports

```tsx
import { Button, DataTable, StatusBadge } from '@shipyard/ui';
import { IconRefresh } from '@tabler/icons-react';
```

Preferred:

```tsx
<Button variant="secondary" icon={<IconRefresh />}>
  Retry failed
</Button>
```

Do not feature-local-style a random native button to look “close enough.”

### Status representation

```tsx
<StatusBadge status="failed">Failed</StatusBadge>
```

The component is responsible for icon, semantic color and accessible text.

---

## 25. Design review checklist

### Product fit

- [ ] The main operator question is obvious.
- [ ] Primary action is obvious but not oversized.
- [ ] Technical metadata is copyable where useful.
- [ ] Provenance is visible for artifacts/releases/deployments.
- [ ] Dense data is a table/list, not cards.
- [ ] Graph has a list/table alternative.

### Visual system

- [ ] IBM Plex Sans / Mono only.
- [ ] Tabler product icons only.
- [ ] Semantic tokens only.
- [ ] 4px spacing system.
- [ ] 6px controls / 8px panels.
- [ ] No product gradients.
- [ ] Shadows only where elevation is real.

### States

- [ ] loading
- [ ] empty
- [ ] error
- [ ] permission denied if applicable
- [ ] disconnected/stale if applicable
- [ ] long-content behavior

### Accessibility

- [ ] keyboard complete
- [ ] visible focus
- [ ] labels/names
- [ ] no color-only state
- [ ] theme contrast
- [ ] reduced motion
- [ ] responsive/mobile monitoring path

### Anti-generic

- [ ] no bento-card dashboard by default
- [ ] no giant hero copy
- [ ] no random pills
- [ ] no sparkle/AI visual cliché
- [ ] no stock shadcn demo composition
- [ ] screen still looks domain-specific without Shipyard logo

---

## 26. Governance

`DESIGN.md` is normative.

Changes to any of the following require a design-system review:
- font family;
- icon family;
- base colors;
- spacing scale;
- control heights;
- radius system;
- navigation structure;
- table interaction model;
- pipeline graph node anatomy;
- core package stack (`react-aria-components`, TanStack Table, React Flow).

When code and this file disagree, either:
1. fix the code; or
2. update this file in the same pull request with a written rationale.

Do not silently drift.

---

## 27. Research references

- **[R1] GitLab Pajamas - Design.** Design tokens, components, patterns, dark-mode review, design-system workflow.  
  https://design.gitlab.com/get-started/design/
- **[R2] GitLab Pajamas - Design Tokens.** Semantic tokens and scalable theme/dark-mode practices.  
  https://design.gitlab.com/product-foundations/design-tokens-using/
- **[R3] GitLab Typography.** Readable sans/mono pairing and restrained typographic hierarchy.  
  https://design.gitlab.com/brand-design/typography/
- **[R4] Grafana Saga - Overview.** Open-source design system as a versioned product with foundations, components and patterns.  
  https://grafana.com/developers/saga/about/overview/
- **[R5] Grafana Saga - Accessibility.** Accessible component behavior and restrained live-region usage.  
  https://grafana.com/developers/saga/foundations/accessibility/accessibility-styleguide/
- **[R6] Nextcloud - Foundations.** Primary-color restraint, light/dark themes, typography and icon guidance.  
  https://docs.nextcloud.com/server/stable/developer_manual/design/foundations.html
- **[R7] Nextcloud - Layout.** Navigation/content/sidebar and list/content information hierarchies.  
  https://docs.nextcloud.com/server/stable/developer_manual/design/layout.html
- **[R8] PatternFly - Typography.** 14px body text, tabular numerals and technical hierarchy.  
  https://www.patternfly.org/foundations-and-styles/typography/
- **[R9] PatternFly - Dark Theme Handbook.** Semantic tokens and light/dark theme discipline.  
  https://www.patternfly.org/developer-resources/dark-theme-handbook/
- **[R10] Harbor UI package.** Current Harbor portal stack identifies Angular and Clarity.  
  https://github.com/goharbor/harbor/blob/main/src/portal/package.json
- **[R11] Gitea UI replacement proposal.** Preference for flexible, low-level/headless UI dependencies rather than a heavy styled framework.  
  https://github.com/go-gitea/gitea/issues/29849
- **[R12] Argo CD UI customization.** Multiple application detail views including tree, pods, network and list.  
  https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/ui-customization.md
- **[R13] Portainer package.** Current hybrid frontend with React migration, Storybook and modern React data tooling.  
  https://github.com/portainer/portainer/blob/develop/package.json
- **[R14] React Aria.** Style-free accessible components, focus management, keyboard and internationalization.  
  https://react-aria.adobe.com/
- **[R15] TanStack Table.** Headless data-grid engine with full control over markup and styling.  
  https://tanstack.com/table/latest
- **[R16] Tabler Icons React.** MIT-licensed React SVG icon package.  
  https://github.com/tabler/tabler-icons/blob/main/packages/icons-react/README.md
- **[R17] IBM Plex.** Open-source Sans/Mono family designed to work in UI environments.  
  https://github.com/IBM/plex/blob/master/README.md
- **[R18] WCAG 2.2 - Target Size Minimum.** Minimum target-size accessibility guidance.  
  https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html
- **[Q1] HN - Density userstyle discussion.** Qualitative power-user discussion: density is valuable when information hierarchy remains clear.  
  https://news.ycombinator.com/item?id=32627675
- **[Q2] HN - High-information-density UI discussion.** Qualitative discussion praising information-dense professional interfaces.  
  https://news.ycombinator.com/item?id=43925732
- **[Q3] HN - Mobile data tables.** Qualitative discussion emphasizing task-focused columns, sticky identity and expandable rows.  
  https://news.ycombinator.com/item?id=45483080


### Research interpretation note

Official project documentation and repositories are used to identify patterns and current technology choices. Hacker News links are included only as qualitative preference signals from technical users; they are not representative usability research and should not override direct Shipyard user testing.
