# Tesseract — Brand Kit

**Version 2.0 · NEON · identity for terminal software**

Brand: `Tesseract` · Command: `ts` · Glyph: `⧉` · Tagline: *the mosaic never falls apart*

---

## 1. The two founding rules

The symbol is **two squares**. That's why the brand has **two colors**, each carrying a fixed meaning.

> **Green is ownership.** Green is never a state color. Green means "your keyboard is here". There's a single phosphor glow lit across the whole screen, in one cell at a time.

> **Cyan is structure.** Cyan is the second dimension — the back square, the grid, the glyph, the numbering. It's also never a state.

Consequence: the state alphabet (▸ ⬤ ⏵ ✖ ○ ⚠) uses neither green nor cyan. No shade, anywhere.

**Neon only works because almost everything is off.** Darkness is the primary material; the glow is the exception. If more than 5% of the screen is lit, the identity has broken.

---

## 2. Where cyberpunk can show up — and where it can't

| Effect | Brand surface<br><small>site, README, banner, docs, social</small> | Product<br><small>inside the terminal</small> |
|---|---|---|
| Green + cyan duotone | yes | yes |
| Glow / bloom | yes | **no** — a terminal doesn't emit light |
| Scanline, CRT vignette | yes | **no** |
| Chromatic aberration | wordmark only | **no** |
| Glitch / jitter | symbol hover only | **no** |
| Digital rain, katakana | **no** | **no** |
| Neon pink as a third color | **no** | **no** — turns it into arcade |

Single reason for every "no" in the product: Tesseract promises **dense, legible, and reliable**. Glow on UI text destroys legible density. Neon is the brand's imagery, not the texture of the work.

---

## 3. Essence

| Field | Definition |
|---|---|
| Essence | A mosaic of agents that never falls apart |
| Promise | Nothing hides behind a tab, nothing is lost when the window closes |
| Archetype | Ruler (order, control) + Creator (instrument of craft) |
| Personality | Dense · Disciplined · Permanent · Alert without shouting |
| Metaphor | A command tower built from mosaic stones |

**Tagline:** `The mosaic never falls apart.`
**Technical one-liner:** `Multiple agents. One grid. One keyboard at a time.`
**Description:** `Terminal panel for running AI agents side by side, with sessions that survive the machine.`

---

## 4. Symbol

Two equal squares offset diagonally (hypercube projection) + a lit tessera at the center of the overlap.

| Rule | Value |
|---|---|
| Front square | 100 × 100, stroke 8, color `brand.phosphor` |
| Back square | identical, offset +24 on X and +24 on Y, color `flux` |
| Tessera | 18 × 18 solid, `fg.bright`, at the center of the overlap |
| Forbidden | border radius, gradient, shadow, fill on the squares |
| Clear space | half the outer square's width, on all sides |

### Character version — canonical, 7×5

This is the version that lives inside the product.

```
┌────┐
│┌───┼┐
││ ▓ ││
└┼───┘│
 └────┘
```

Terminal coloring: back square in `color14` (cyan), front square in `color2` (green), `▓` in `color10` (phosphor).

### Single glyph

`⧉` (U+29C9) — prompt, window title, badge, anywhere with room for one character.

### Degradation

| Size | What to show |
|---|---|
| ≥ 48px | Full symbol, duotone, tessera |
| 24–47px | Full, proportionally thicker stroke |
| 16px (favicon) | Just the two squares, monochrome, **no** tessera |
| 1 character | `⧉` |
| Inside the app | 7×5 box-drawing version |

---

## 5. Palette v2.0

### Darkness

| Token | Hex | Use |
|---|---|---|
| `bg.void` | `#030507` | TYPE mode background |
| `bg.base` | `#070B0C` | Default background |
| `bg.surface` | `#0C1315` | Cell body |
| `bg.raised` | `#121C1F` | Header, selection, Docker panel |
| `line.dim` | `#16282A` | Unfocused grid |
| `line.active` | `#205047` | Focused project grid |

### Text

| Token | Hex | Use |
|---|---|---|
| `fg.faint` | `#3E534E` | Off, help, shortcuts |
| `fg.muted` | `#6C8076` | Secondary text |
| `fg.default` | `#BFD1C6` | Primary text |
| `fg.bright` | `#E8F4EC` | Titles, tessera |

### Neon — green is ownership

| Token | Hex | Use | Frequency |
|---|---|---|---|
| `brand.deep` | `#0B3322` | Badge background | free |
| `brand.core` | `#1F7A4C` | Active project grid, logo | free |
| `brand.live` | `#35C27A` | Focused cell in NAVIGATE | 1 per screen |
| `brand.phosphor` | `#55FFA6` | **Keyboard owner.** Cursor, TYPE border, badge, tessera | **1 per screen** |

### Neon — cyan is structure

| Token | Hex | Use |
|---|---|---|
| `flux.deep` | `#082F31` | Label backgrounds |
| `flux.core` | `#128C86` | Grid, corners, labels |
| `flux` | `#22E0D0` | `⧉` glyph, numbering, back square |

### States — no green, no cyan

| Signal | State | Token | Hex |
|---|---|---|---|
| `▸` | working | `state.working` | `#6C8076` |
| `⬤` | responded | `state.read` | `#7DB7E8` |
| `⏵` | approve | `state.block` | `#FFB454` |
| `✖` | crashed | `state.dead` | `#FF3B47` |
| `○` | stopped | `state.off` | `#3E534E` |
| `⚠` | orphaned | `state.orphan` | `#C77DFF` |

---

## 6. Responded ≠ Approve

The heart of the product. The difference **cannot depend on color or brightness**.

**Rule: urgency is filled area, not hue.**

| | Responded | Approve |
|---|---|---|
| Shape | dot `⬤` | triangle `⏵` |
| Area | small glyph | **solid bar across the full line** |
| Video | normal | **inverted** |
| Motion | static | blinks every 2s |
| Color | ice blue | amber |

Test: with `NO_COLOR=1`, the `approve` line remains the only solid bar on screen.

---

## 7. The two modes

### NAVIGATE

- Background `bg.base`, cells in `bg.surface`
- Grid in single stroke `┌ ─ ┐ │ └ ┘`, in `flux.core`
- Focused project in `line.active`, the rest in `line.dim`
- Selected cell with `brand.live` border
- All states visible

### TYPE

- Background drops to `bg.void`; everything but the active cell goes to `fg.faint`
- Active cell keeps `bg.surface` and gets a **double border** `╔ ═ ╗ ║ ╚ ╝` in `brand.phosphor`
- Inverted badge in the top-right corner: `▓ TYPE ▓` (phosphor background, `bg.void` text)
- Block cursor, `brand.phosphor`

**Rule:** single stroke = you command the app. Double stroke = the cell commands the keyboard. Works without color.

---

## 8. Typography

| Role | Font | Note |
|---|---|---|
| Display / wordmark | **Martian Mono** 800 | Uppercase only, tracking +.24em, with 2px chromatic aberration |
| Product | **Iosevka Term** | Narrow = more columns per cell |
| Free alternative | **JetBrains Mono** | If Iosevka is too dense |
| Paid alternative | **Berkeley Mono** | Character, if you're willing to pay |

**Don't use:** Fira Code, Cascadia, Inter, anything with arrow ligatures — ligatures scramble grid alignment.

Hierarchy inside the app:

| Level | Style |
|---|---|
| Project name | Uppercase, `fg.bright`, tracking +1 |
| Cell name | `fg.default` |
| State / metadata | `fg.muted` |
| Help / shortcut | `fg.faint` |

---

## 9. Applications

### Boot banner

```
   ┌────┐
   │┌───┼┐    T E S S E R A C T
   ││ ▓ ││    the mosaic never falls apart
   └┼───┘│
    └────┘    ts 0.1.0 // MIT

   8 cells recovered // 3 projects // 41ms
```

### Other surfaces

| Surface | Form |
|---|---|
| Window title | `⧉ ts — api/claude-refactor` |
| Prompt | `⧉ ~/projects/api ›` |
| README badge | `⧉ tesseract // MIT` |
| Favicon | two squares, solid, no tessera |
| Light background (GitHub README) | solid `#070B0C` symbol, no neon — light green on white doesn't read |

---

## 10. Tone of voice

Direct, technical, imperative, with the shortcut attached. No exclamation marks, no emoji, no first-person plural.

| We are | We are not |
|---|---|
| Precise | Cold |
| Short | Terse |
| Technical | Hermetic |
| Calm on error | Dramatic |

### Real microcopy

```
No cells in this project yet. `n` creates the first one.
Blocked, waiting on you. `Enter` approves, `Esc` declines.
Crashed with exit 1. `r` restarts in the same spot.
8 cells recovered. Same position.
Orphaned cell: process alive, project gone. `m` moves it, `k` kills it.
```

### Anti-examples

```
Oops! Something went wrong 😅
Loading your magic...
ACCESS GRANTED, RUNNER.
No items found.
```

The third is the new risk in v2: **neon in the imagery doesn't authorize cosplay in the copy**. The voice stays dry.
The fourth fails for a different reason: it describes the emptiness without giving the way out. Every empty state ends on a key.

---

## 11. Do & Don't

**Do**

1. Darkness first — neon only exists because 95% of the screen is off.
2. Green = ownership, cyan = structure. Never reversed.
3. Glow and scanline only on the brand surface, never in the terminal.
4. Distinguish states by shape and area before color.
5. A single phosphor glow lit per screen.
6. Test everything with `NO_COLOR=1` before approving.

**Don't**

1. Green or cyan as a state signal.
2. Digital rain, decorative katakana, "ACCESS GRANTED".
3. Glow on UI text.
4. More than one inverted badge visible at once.
5. Neon pink as a third color.
6. The word "tesseract" alone next to OCR or imagery.

---

## 12. ANSI 16 — the base of the colorscheme

```
background      #070B0C
foreground      #BFD1C6
cursor          #55FFA6
cursor_text     #030507
selection_bg    #121C1F
selection_fg    #E8F4EC

color0   black    #070B0C
color1   red      #C22F38
color2   green    #1F7A4C
color3   yellow   #C9A227
color4   blue     #3E7FA8
color5   magenta  #8B4FC4
color6   cyan     #128C86
color7   white    #BFD1C6

color8   black    #3E534E
color9   red      #FF3B47
color10  green    #55FFA6
color11  yellow   #FFB454
color12  blue     #7DB7E8
color13  magenta  #C77DFF
color14  cyan     #22E0D0
color15  white    #E8F4EC
```

### Tokens → ANSI

| Token | ANSI |
|---|---|
| `brand.core` | `color2` |
| `brand.phosphor` | `color10` |
| `flux.core` | `color6` |
| `flux` | `color14` |
| `state.read` | `color12` |
| `state.block` | `color11` |
| `state.dead` | `color9` |
| `state.orphan` | `color13` |
| `state.working` | `color7` |
| `state.off` | `color8` |

---

## 13. Implementation checklist

- [ ] 20 tokens as constants in the code, no loose hex
- [ ] Grid in single stroke (NAVIGATE) and double (TYPE)
- [ ] `brand.phosphor` only accessible through the active cell's component
- [ ] `approve` renders an inverted bar, not just a colored glyph
- [ ] Boot banner with the 7×5 duotone mark
- [ ] Prompt and window title with `⧉`
- [ ] Run with `NO_COLOR=1` — states remain distinguishable
- [ ] Test on a 16-color terminal (default WSL)
- [ ] Test the README on GitHub's light theme
