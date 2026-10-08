# Theme and brand

The source of truth is the brand sheet, [`style/tesseract-brand-identity.html`](../style/tesseract-brand-identity.html)
— *Brand Identity v2.0 — neon*. Everything the product draws follows it, and this page says
how each rule of the sheet became a rule of the screen.

Color here isn't decoration, it's grammar. It has three laws, and none of them is
aesthetic:

- **Green means keyboard ownership.** Never a state. Phosphor green appears on the one
  thing holding your keyboard — a cell in TYPE, or the caret of the field you're writing —
  and nowhere else.
- **Cyan is structure.** The rule at the top, the corners of every box, labels, numbering,
  keys, the `⧉` glyph. Never a state.
- **State doesn't use green or cyan**, and urgency is filled area, not hue.

Beyond that: no glow, no scanline, no chromatic aberration, no digital rain, no neon pink,
no ligature, no emoji, no rounded corner inside the terminal. Those effects only exist on
the brand surface — the README, the site, the banner.

## The palette

The whole palette lives in **a single file**, `internal/theme/theme.go`. No other file in
the project writes a color: whoever draws asks for the token by name
(`theme.BrandPhosphor`, `theme.FluxCore`, `theme.StateBlock`).

| Token | Hex | Where the screen uses it |
|---|---|---|
| `bg.void` | `#030507` | the whole screen in TYPE |
| `bg.base` | `#070B0C` | the whole screen in BROWSE; text on the `approve` bar |
| `bg.surface` | `#0C1315` | the body of a cell and of a dialog |
| `bg.raised` | `#121C1F` | the header of a cell and of a dialog; the selected row; marked text |
| `line.dim` | `#16282A` | the divider of an unfocused project; every line while a cell has the keyboard |
| `line.active` | `#205047` | the lines of a cell |
| `fg.faint` | `#3E534E` | what is off: paths, everything else in TYPE |
| `fg.muted` | `#6C8076` | secondary text: hints, what a key does, an unfocused cell's content |
| `fg.default` | `#BFD1C6` | primary text |
| `fg.bright` | `#E8F4EC` | titles: the wordmark, the focused project and cell |
| `brand.core` | `#1F7A4C` | the divider of the focused project; the symbol's front square |
| `brand.live` | `#35C27A` | the line of the focused cell and of a dialog; the bar of a selected row |
| `brand.phosphor` | `#55FFA6` | the cell in TYPE, its badge, the caret; the symbol's tessera |
| `flux.core` | `#128C86` | corners, labels, the rule at the top, the mode's name |
| `flux` | `#22E0D0` | the `⧉` glyph, numbering, keys; the symbol's back square |
| `▸ working` | `#6C8076` | deliberately dim |
| `⬤ responded` | `#7DB7E8` | something to read |
| `⏵ approve` | `#FFB454` | blocking — always an inverted bar |
| `✖ crashed` | `#FF3B47` | dead process |
| `○ stopped` | `#3E534E` | empty outline |
| `⚠ orphaned` | `#C77DFF` | anomaly |

The guard of the single-file rule is executable:

```bash
./scripts/check-theme.sh
```

It prints the whole palette in ANSI blocks for a visual check, and **fails** if anyone uses
green or cyan as a state color, or writes a color outside the theme file — a hex, or a
color escape code typed by hand.

## The two modes

A single line means you command the application. A double line means the cell commands the
keyboard. The mode changes four things at once, so it's impossible to mistake:

| | BROWSE | TYPE |
|---|---|---|
| Screen background | `bg.base` | `bg.void` |
| Focused cell | single line, `brand.live` | **double** line, `brand.phosphor` |
| Top right corner | the mode's name, in cyan | the `▓ TYPE ▓` badge, inverted in phosphor |
| Everything else | lit | dimmed: no background, faint text, `line.dim` |

That badge is the **one** inverted badge the screen has room for. Tabs, counters and
headings are never inverted: the active tab is cyan and bold, a counter wears its state's
color, a heading is a label.

## States

Shape and area come before color. Each state has its own glyph, and only one of them fills
area:

| Signal | Shape | Area |
|---|---|---|
| `▸ WORKING` | arrow | glyph and label |
| `⬤ RESPONDED` | dot | glyph and label |
| `⏵ APPROVE` | triangle | **solid inverted bar across the whole header line**, blinking every 2 s — the dim frame drops the fill to half strength (`#835F30`, ANSI 3 on 16 colors) and stays a bar |
| `✖ CRASHED` | cross | glyph and label |
| `○ STOPPED` | outline | glyph and label |
| `⚠ ORPHANED` | warning | glyph and label |

Nothing else on the screen is a filled area in a state color. A selection is a raised row
with a bar on its left, marked text is the raised background, a title in the markdown
reader is a heading — never a bar.

## Color profiles

The theme has three profiles and picks on its own:

| Profile | When | What happens |
|---|---|---|
| Full color | the default | the hex codes go out as they are; a 256-color terminal gets them downgraded by the renderer |
| 16 colors | `TERM` contains `16color` | each token falls on the index of the brand's ANSI table (// 10) |
| No color | `NO_COLOR` set, or `TERM=dumb` | bold, underline and reverse video only |

In all three the state alphabet stays legible, because the glyph and the shape carry the
meaning and color only reinforces it. With no color, the `approve` bar is the **only**
thing in reverse video: the mode's badge keeps its blocks and its weight, and marked text
is underlined.

On 16 colors the symbol is `color14` + `color2` + `color10`, and the raised background has
no index of its own — it falls flat, and the bar and the weight are what mark a selection.
The grid's line lands on cyan (6), never on green: green there is still keyboard ownership.

The terminal's own background, text and cursor are only imposed in full color (`bg.base`,
`fg.default`, and a phosphor cursor while a cell holds the keyboard — all three handed back
on exit, the cursor as soon as `ctrl+l` returns the keyboard). On 16 colors the brand
defines indexes, not shades, so the terminal's palette is left alone; with no color nothing
is imposed at all.

Code blocks in the markdown reader are highlighted in text and structure only — the `fg.*`
ladder and the two cyans. No state color and no green tints code.

| Variable | What it does |
|---|---|
| `NO_COLOR` | strips all color; leaves bold, underline, reverse video and border |
| `TESSERACT_NO_BLINK` | stops the `approve` bar's blink, keeping the bar |
| `TESSERACT_NO_OPENING` | skips the startup animation and goes straight to the grid |

## Where the brand shows up

The symbol doesn't only live in the README. Inside the product it has four homes:

| Place | Form |
|---|---|
| Startup | the symbol drawing itself, stroke by stroke, on launch |
| Title bar | the `⧉` glyph, always to the left of the name |
| Empty grid | the 7×5 symbol at the center, with the key that creates the first cell |
| Window title | `⧉ ts — project/cell`, following the focus |

The character version is the canonical one, 7×5: the back square in cyan from the top
left, the front one in green, the tessera in phosphor.

```
┌────┐
│┌───┼┐
││ ▓ ││
└┼───┘│
 └────┘
```

### The startup

The brand doesn't appear ready-made at launch: **it assembles itself**. The back square is
born first, stroke by stroke, with the pen tip lit up in phosphor running along the path.
Then the front square on top. The tessera lights up, the whole symbol bursts in a single
frame and settles. Only then does the name open, letter by letter, and the engine's tally
closes the block.

```
   ┌────┐
   │┌───┼┐    T E S S E R A C T
   ││ ▓ ││    the mosaic never falls apart
   └┼───┘│
    └────┘    ts 0.1.0 // MIT

   8 cells recovered // 3 projects // 41ms
```

It's the brand sheet's boot banner (// 07), line for line. On a grid with nothing to bring
back — a first run — the tally says so instead of counting zeros: `no cells to recover //
the first one opens here // 41ms`.

Lasts about a second and a half, and **runs in parallel with the connection** — while the
brand draws itself, the engine is searched for and the grid is rebuilt on the other
execution thread. The stopwatch at the end of the tally only counts the engine, never the
animation: it's a number that exists to be true.

None of this is glow, scanline or glitch — the terminal doesn't emit light and the rule
still holds. What moves is the **order in which things come into existence**, not their
texture. Without a real terminal (output to a file, a pipe), the startup becomes the static
block, written once. In a terminal narrower than the block (42 columns) the static block
stacks: the signature goes under the symbol and the tally breaks into one figure per line,
so nothing wraps over the drawing. The banner is written outside the screen's program, and
it goes through the same color downgrade the grid gets: a 256-color terminal never receives
a 24-bit code from it.

## Tone of voice

The microcopy is dry and ends in a key: `No cells yet. n creates the first one.` It never
apologizes, never celebrates and never plays a character — neon in the imagery doesn't
authorize cosplay in the copy. An empty state says what is empty and which key changes
that; an error says what failed, after the `✖` that tells it apart with no color.

## The same theme across the rest of the desk

The `themes/` folder brings **Tesseract Neon** ready for the terminal and the everyday
tools:

| File | For |
|---|---|
| `windows-terminal.json`, `wezterm.toml`, `alacritty.toml`, `kitty.conf`, `ghostty` | terminal emulators |
| `tesseract-neon.yaml` | base16/base24 scheme (tinted-theming) |
| `tmux.conf`, `starship.toml`, `fzf.env` | status bar, prompt, search |
| `bat.tmTheme`, `delta.gitconfig` | file reading and diff |
| `nvim/tesseract.lua`, `eza-ls-colors.sh` | editor and listing |
| `claude-code.md` | Claude Code inside the cell, without its own color |

Every one of them carries the brand's ANSI table and `background` `#070B0C`, `foreground`
`#BFD1C6`, `cursor` `#55FFA6`, `cursor_text` `#030507`, `selection` `#121C1F`. The prompt
in `starship.toml` is the sheet's: `⧉ ~/projects/api ›` — glyph in cyan, path in the
secondary tone, `›` in phosphor.

**The agent inside the cell.** Claude Code paints with its own palette and clashes with the
grid — orange on the badge, pink on the logo. The fix is one line: `/config` → theme →
**`dark-ansi`**. In that theme it draws only with the 16 ANSI colors, which are Tesseract
Neon's. Details in [`themes/claude-code.md`](../themes/claude-code.md).

The brand in vector form is at `themes/logo.svg` (colored) and `themes/logo-mono.svg`
(single stroke in `currentColor`, for favicon and 16px). The details of each file are in
[`themes/README.md`](../themes/README.md).
