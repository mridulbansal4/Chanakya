# CHANAKYA — Design System (Master)

The global source of truth for CHANAKYA's interface. Page-specific overrides,
if any, live in `design-system/chanakya/pages/<page>.md` and take precedence
over this file.

The implementation is `frontend/packages/ui/src/styles/globals.css`. This
document explains the reasoning; that file is the executable form of it. If
the two disagree, the CSS is correct and this document is stale.

---

## 1. The one rule

**No component may contain a hex value or a raw Tailwind palette class.**

Not `#11131C`, not `bg-blue-600`, not `text-slate-400`, not `border-white/10`.
Every colour comes from a semantic token.

This is the rule the previous system had and did not enforce, and the cost was
concrete: 66 hardcoded hex values and ~120 palette literals across 10 files
meant the token layer described a design nobody was actually rendering. A
palette that can be bypassed is documentation, not a system.

Graph components are the tempting exception, because React Flow needs real
colour strings for SVG strokes. They are handled by `lib/graph-theme.ts`, which
exports `var(--token)` references — real CSS values that resolve in the DOM, so
the graph cannot drift from the app.

---

## 2. Colour

### Canvas

A near-neutral ramp with a ~4° cool cast. Deliberately not blue-tinted: a
tinted canvas reads as *tech product*, a neutral one reads as *instrument*.

| Token | Value | Use |
|---|---|---|
| `--sunken` | `#08090A` | Wells, graph canvas, app chrome |
| `--canvas` | `#0B0C0E` | Page background |
| `--raised` | `#121317` | Cards, panels, table surfaces |
| `--overlay` | `#16181D` | Dialogs, popovers, command palette |
| `--elevated` | `#1B1E24` | Hover fill on raised surfaces |

Steps are close together on purpose. Elevation should be *felt*, not read as
banding.

### Foreground

Every step above `--fg-faint` is verified against `--canvas` at ≥ 4.5:1.

| Token | Value | Contrast | Use |
|---|---|---|---|
| `--fg` | `#F2F4F7` | 17.8:1 | Primary text |
| `--fg-muted` | `#A8B0BC` | 8.9:1 | Secondary text, descriptions |
| `--fg-subtle` | `#7B8494` | 5.2:1 | Eyebrows, column headers, tertiary |
| `--fg-faint` | `#5A6373` | 3.3:1 | **Decorative and disabled only** |

`--fg-faint` fails 4.5:1. It is legal on icon glyphs, placeholder text and
disabled controls. It is never legal on a label a user has to read. The old
`text-white/40` (≈2.5:1) and `text-white/50` (≈3.4:1) were doing exactly that.

### Colour carries five meanings, and no others

| Token | Meaning |
|---|---|
| `--accent` | Interactive, brand, provenance |
| `--ok` | Verified, in force, covered |
| `--warn` | Needs human judgement, pending, drift |
| `--risk` | Gap, violation, rejected |
| `--info` | Aliased to accent |

Each has `-weak` (12% tint, for fills) and `-line` (30% tint, for borders).

Status colours are desaturated relative to stock Tailwind. In an instrument,
alarm comes from contrast against a calm field, not from saturation. If
everything is vivid, nothing is urgent.

**Nothing is coloured for decoration.** The clearest violation in the previous
build was the command palette assigning emerald / cyan / purple / amber to nine
navigation entries — nine colours carrying zero information, in a system where
those same hues mean "verified" and "needs judgement" elsewhere. Spending a
semantic colour on decoration is what makes the real signal stop registering.

Accent is a single hue with two stops: `--accent` (#4C82F7, 5.5:1 — for text,
icons, strokes) and `--accent-solid` (#1D4ED8, 6.7:1 with white — for fills).
The previous build had `--steel-blue` in tokens competing with `blue-600`,
`blue-500`, `blue-400` and `indigo-500` in markup.

### Never colour alone

Colour is never the sole carrier of meaning. `StatusDot` encodes state in
**shape** as well as hue — approved is filled, needs-review is ringed, pending
is hollow. Semantic edges in the blast graph are **dashed**, not just amber.
This survives greyscale, colour vision deficiency, and a printed audit pack —
which for a regulatory system is not a hypothetical.

---

## 3. Type

Three families doing genuinely different jobs. The previous pairing
(Plus Jakarta Sans + Inter) put two humanist sans faces side by side, so the
display/body split created no hierarchy at all.

| Family | Variable | Job |
|---|---|---|
| **Source Serif 4** | `--font-serif` | Page titles, dialog titles, empty-state headings — **only** |
| **Inter** | `--font-sans` | All interface and data text |
| **JetBrains Mono** | `--font-mono` | Clause refs, IDs, hashes, aligned figures |

The serif is the app's one editorial gesture. Used sparingly it signals
"document of record". Used inside tables and cards it is just noise — a serif
at 13px in a dense table is decoration, not authority.

### Scale

- `.text-display-*` — serif, weight 400. Page titles.
- `.text-headline-*` — sans, weight 600. Section and card headings.
- `.text-title-*` — sans, weight 600. Dense headings, table headers.
- `.text-body-*` — sans, weight 400. Prose.
- `.text-label-*` — sans, weight 500–600. Controls, chips, eyebrows.
- `.text-metric-*` — sans, weight **550**, tabular figures, −0.02em.

**Numbers are never set in the display face and never above weight 600.** A KPI
at weight 800 reads as advertising, not as data. The previous overview used
`text-display-md` (2.8rem / weight 800) for posture figures.

Prose uses proportional figures; anything that must align in a column opts into
`.tnum`. Tabular figures are not applied globally — they are wider and looser,
and they make running text worse.

Measure is capped at ~68ch for descriptions. Line length governs whether a
paragraph gets read more than typeface choice does.

---

## 4. Space, radius, elevation

**Radius: 8px base** (`--radius: 0.5rem`). Heavily rounded corners read
consumer; institutional software sits between 4 and 8. The previous value was
0.85rem (13.6px) with `rounded-2xl`/`rounded-3xl` scattered on top.

**Elevation: three neutral levels.** `--elev-1/2/3`, plus `--highlight` — a 1px
inner top highlight that does most of the work of making a surface read as
raised, without shadow noise.

No coloured glow. The previous `0 0 25px rgba(37,99,235,.15)` card-hover glow is
the glassmorphism-showcase look, and it appeared on hover of every card.

Shadows describe elevation. A drop shadow on a flush element is a shadow cast
by nothing — the old KPI strip carried `shadow-2xl` while sitting flat against
the chrome.

---

## 5. Motion

Defined once in `lib/motion.ts`, mirroring the `--dur-*` and `--ease-*` tokens.

| Token | Duration | Use |
|---|---|---|
| `DUR_MICRO` | 120ms | Colour, opacity, hover |
| `DUR_STANDARD` | 180ms | Dropdowns, popovers, small enter/exit |
| `DUR_STRUCTURAL` | 260ms | Page, dialog, layout |

`EASE_OUT` = `cubic-bezier(0.2, 0.8, 0.2, 1)` — fast departure, soft arrival.
Springs are reserved for things that physically move between two positions (the
nav indicator, the view toggle). Everything else uses a duration curve, which is
more predictable and cheaper.

Principles:

- **Motion answers "where did this come from" or "where did it go".** If it
  answers neither, delete it.
- **Travel is small.** Page enter is 4px. On a screen navigated all day, a
  larger travel distance stops reading as polish and becomes waiting.
- **Buttons and cards do not translate on hover.** Hover is a fill and border
  change. A −2px lift on every element means the interface twitches as the
  pointer crosses it.
- **Stagger is capped** (`staggerDelay`, 240ms ceiling). An uncapped
  `index * step` means the 40th row waits two seconds — the stagger stops being
  a flourish and becomes latency.
- **Perpetual animation needs a reason.** Only provenance edges animate in the
  overview graph; marching ants on every edge is motion that never stops and
  carries nothing.
- **`prefers-reduced-motion` is a global stop** in `globals.css`, not a
  per-component opt-in.

---

## 6. Accessibility (non-negotiable)

- **Focus is never removed.** One treatment: 2px `--accent` outline, 2px offset,
  applied via `:focus-visible` globally.
- **Contrast ≥ 4.5:1** for all text. See the foreground table.
- **Interactive elements are real elements.** The account control was a `<div>`
  with `cursor-pointer` — visually interactive, unreachable by keyboard,
  announced as nothing. It is a `<button>` with an accessible name.
- **Dialogs have the full contract:** `role="dialog"`, `aria-modal`, focus trap,
  focus restore to the trigger, Escape to close, body scroll lock.
- **The command palette is a combobox + listbox** with `aria-activedescendant`,
  roving selection, `scrollIntoView({block:"nearest"})` on the active row, and a
  live region announcing the result count.
- **Loading states announce.** `LoadingRegion` wraps skeletons in
  `role="status"` / `aria-busy`.
- **Camera moves announce.** Graph search outcome goes to a live region,
  because a viewport change is invisible to a screen reader.
- **Touch targets ≥ 44px** for primary touch actions (`size="lg"`). Dense
  desktop controls sit at 32–36px; small icon buttons grow their hit area with
  negative margin (`-m-1.5 p-1.5`) rather than growing the layout.
- **Zoom is never disabled.** No `maximum-scale`, no `user-scalable=no`.
- **Skip link** to `#main-content` as the first focusable element.

---

## 7. What this is not

CHANAKYA should not read as: startup dashboard, AI wrapper, Tailwind template,
glassmorphism showcase, Material demo, Dribbble concept.

Concretely, that means avoiding: rainbow categorical colour, gradient fills as
decoration, blue glow on hover, backdrop blur over scrolling data, heavy
corner rounding, weight-800 numerals, translucent chrome, and animation that
runs forever without meaning.

Reference register: Linear, Palantir Foundry, Bloomberg Terminal, Stripe docs,
Vercel.

---

## 8. Status

**Migrated** to the new system directly: `globals.css`, `layout.tsx`,
`lib/motion.ts`, `lib/graph-theme.ts`, `button.tsx`, `app-shell.tsx`,
`app/page.tsx`, `command-palette.tsx`, `page-header.tsx`, `badges.tsx`,
`skeleton.tsx`, `empty-state.tsx`, `screen-banner.tsx`, `page-transition.tsx`,
`confidence.tsx`, `overview-hierarchy.tsx`, `overview-graph.tsx`,
`blast-graph.tsx`, `lineage-graph.tsx`, `graph-legend.tsx`, `graph-search.tsx`.

**Inherited** via legacy token aliases plus a mechanical radius/elevation/weight
normalisation — correct colours and geometry, but not individually recomposed:
`register`, `evidence`, `policy`, `review`, `audit`, `feed`, `amendments`,
`regulatory-feed`, `welcome-modal`, `glossary-modal`, `signoff-modal`,
`obligation-detail`, `amendment/kit`, `amendment/steps`, `help-menu`,
`as-of-control`, `health-indicator`.

The legacy aliases at the bottom of `:root` in `globals.css` (`--surface`,
`--cream-200`, `--text-dim`, `--lavender`, …) exist only to keep those screens
rendering against the new palette. **Do not use them in new code.** They should
be deleted once the list above is empty.

`app/ui-demo/page.tsx` is leftover scaffolding documenting Kibo/Forge UI
installation. It is not linked from navigation and should probably be deleted.
