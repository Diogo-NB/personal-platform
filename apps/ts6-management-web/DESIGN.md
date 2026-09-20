# DESIGN.md - TeamSpeak Control

## Context (from discovery)

- Artifact type: settings/admin control application
- Positioning: technical and utilitarian
- Audience: the private TeamSpeak operator | Primary action: safely reconcile the singleton server state
- Adjectives: controlled, legible, industrial, calm, exact
- Visual word translations: controlled -> strict state-dependent actions; legible -> oversized state and plain labels; industrial -> rack-panel palette and defined edges; calm -> restrained motion and generous gaps; exact -> tabular telemetry and explicit lifecycle sequence
- Aesthetic essence (3 words): quiet industrial precision
- Single-minded proposition: the operator can identify the server state and issue the one valid command without ambiguity
- References: admire broadcast rack faceplates for hierarchy, Linear for interaction speed, and Stripe Dashboard for calm operational data; avoid generic SaaS cards and decorative cyberpunk consoles
- Mode: dark | Density: balanced
- Constraints: vanilla TypeScript and CSS, no UI framework, WCAG 2.2 AA, responsive from 320px, no API-key persistence

## Aesthetic

- Direction: broadcast control-room faceplate adapted into a compact web console
- Defining trait: one oversized service-state readout anchors an asymmetrical status-and-lifecycle grid
- Signature move: a four-position lifecycle track acts as a physical signal path beside the live state word

## Typography

- Display: Bricolage Grotesque Variable | source: Fontsource/Google Fonts | license: OFL-1.1
- Body: IBM Plex Sans Variable | source: Fontsource/Google Fonts | license: OFL-1.1
- Scale: ratio 1.125 Major Second, base 16px | xs 12/1.5 labels | sm 14/1.5 metadata | body 16/1.6 copy | lg 18/1.6 lead | xl 20.25/1.35 | 2xl 22.78/1.25 | 3xl 28.83/1.15 | fluid display 60.8-131.2/0.94 live state
- Weights: 400/600/650 | Measure: 65-75ch | Tracking notes: display -0.02em; operational labels +0.09em

## Color

- Strategy: near-black blue-green machinery neutrals with a sparing amber control accent, outside the default indigo band
- Distribution: 60 neutral / 30 blue-green surfaces / 10 amber and semantic signals
- Palette (role -> OKLCH | hex):
  - bg: oklch(0.165 0.018 220) | #0b1115
  - surface: oklch(0.205 0.021 220) | #111a20
  - fg: oklch(0.955 0.012 135) | #eef2ed
  - muted: oklch(0.72 0.02 195) | #9ca9a8
  - border: oklch(0.365 0.025 215) | #35434a
  - accent: oklch(0.82 0.16 75) | #ffb341
  - accent-fg: oklch(0.22 0.04 75) | #1d1608
  - success: oklch(0.79 0.14 155) | #64d69a
  - warning: oklch(0.82 0.16 75) | #ffb341
  - error: oklch(0.75 0.16 28) | #ff8c7d
- Dark mode overrides: dark-only interface; elevation steps from bg L 0.165 to surface L 0.205 and raised L 0.25

## Spacing, radius, shadow

- Spacing base: 4px, scale: 1, 2, 3, 4, 6, 8, 12, 16
- Radius: 4px controls and 10px surfaces
- Shadow approach: defined edge only; no elevation shadows

## Layout and composition

- Grid: asymmetrical two-column operational grid | gutters/margins: 16px mobile, fluid 48-128px internal gap desktop
- Spacing rhythm: 8-16px within groups, 32-64px between regions
- Signature layout move: the state word shares the first viewport with a narrow lifecycle signal track instead of a card grid
- Density: balanced | Scanning: F
- Responsive: mobile-first behavior with desktop composition | breakpoints: 32rem and 48rem

## Components and states

- Button hierarchy: primary amber fill / secondary defined edge / tertiary text; hover translates 1px, active returns 1px, focus cyan 3px ring, disabled muted, loading disabled with stable label width
- Inputs: visible label, password type, validation on submit, textual live-region error that preserves input
- Tables: not used; telemetry uses tabular numerals and left-aligned labels
- Overlays: none
- Empty / loading / error: authentication button says Checking; missing status becomes an unavailable state; request failures remain in a live feedback surface
- Focus ring: 3px cyan box-shadow with no layout shift

## Motion

- Duration scale: fast 120ms / normal 220ms
- Easing: cubic-bezier(0.16, 1, 0.3, 1)
- What animates: button transform and control color only | reduced-motion: transform removed and durations reduced to 1ms
- Signature motion: none; two-second telemetry must feel stable

## Iconography

- Set: typographic marks only | grid: 24px | stroke: 1-2px defined edges | caps/joins: square | radius match: yes

## Imagery and illustration

- Mode: no imagery; the real product status is the visual subject
- Rules: no decoration that competes with operational state
- Avoid: stock operations imagery, abstract gradients, faux terminal ornament
- Text-over-image contrast: not applicable

## Dark mode

- Base bg: L 0.165 | fg: off-white L 0.955 | elevation ramp: L 0.165, 0.205, 0.25
- Accent: amber L 0.82 with dark foreground | border: L 0.365

## Accessibility

- Contrast: AA-targeted dark mode | Focus: visible and never covered by sticky UI
- Keyboard: native controls and logical DOM order | Targets: 44px | Color independence: state word and lifecycle label accompany every color | Reduced motion: yes
- Notes: live feedback uses polite announcements; lifecycle uses aria-current; API-key field has an associated label and allows paste

## Tokens (source of truth)

```css
:root {
  --font-display: "Bricolage Grotesque Variable", sans-serif;
  --font-body: "IBM Plex Sans Variable", sans-serif;
  --space-1: 0.25rem;
  --space-2: 0.5rem;
  --space-3: 0.75rem;
  --space-4: 1rem;
  --space-6: 1.5rem;
  --space-8: 2rem;
  --space-12: 3rem;
  --space-16: 4rem;
  --radius-sm: 0.25rem;
  --radius-md: 0.625rem;
  --bg: oklch(0.165 0.018 220);
  --surface: oklch(0.205 0.021 220);
  --fg: oklch(0.955 0.012 135);
  --muted: oklch(0.72 0.02 195);
  --border: oklch(0.365 0.025 215);
  --accent: oklch(0.82 0.16 75);
  --success: oklch(0.79 0.14 155);
  --warning: oklch(0.82 0.16 75);
  --error: oklch(0.75 0.16 28);
}
```

- Adapter: plain CSS

## Cards and surfaces

- Cards/surfaces: defined edge or lightness shift, never shadow, 10px maximum radius, 24-32px desktop padding | nesting: no cards in cards

## Slop audit

- Date: 2026-09-19 | Result: pass from source audit
- Notes: no indigo, gradients, glow, nested cards, side-tab borders, inflated radii, stock imagery, or decorative motion. Full component states, 44px targets, visible labels, keyboard focus, semantic status text, reduced motion, responsive reflow, and build correctness are covered. Render-level inspection remains part of local acceptance.

## Changelog

- 2026-09-19: established the v1 private operations console as a dark broadcast-control faceplate with Bricolage Grotesque, IBM Plex Sans, amber signals, and a lifecycle-track signature.
