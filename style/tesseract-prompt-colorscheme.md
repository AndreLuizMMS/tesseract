# Prompt — Tesseract Neon: colorscheme + surfaces

Paste the block below, whole, into an agent (Claude Code, Cursor, etc.) at the root of the Tesseract repository. It's self-contained: it carries the full palette and doesn't depend on any other file.

Before pasting, adjust the two lines marked with `<<<` in the CONTEXT section.

---

````
You are a design systems engineer specializing in terminal tools.
Generate the Tesseract project's theme artifacts from the specification below.
Don't invent colors. Don't change hex values. Don't add new colors.

## CONTEXT

Tesseract is a terminal panel where several AI agents run side by side
in a grid. Command: `ts`. Open source, MIT. Documentation in English.
Project language/stack: <<< FILL IN (e.g. Go + Bubble Tea / Rust + Ratatui)
Output directory: <<< FILL IN (e.g. ./themes)

## CANONICAL PALETTE — single source of truth

### Base
bg.void          #030507   TYPE mode background
bg.base          #070B0C   default background
bg.surface       #0C1315   cell body
bg.raised        #121C1F   header, selection
line.dim         #16282A   unfocused grid
line.active      #205047   focused project grid
fg.faint         #3E534E   off, shortcuts
fg.muted         #6C8076   secondary text
fg.default       #BFD1C6   primary text
fg.bright        #E8F4EC   titles

### Neon — green is KEYBOARD OWNERSHIP
brand.deep       #0B3322
brand.core       #1F7A4C   active project grid, logo
brand.live       #35C27A   focused cell
brand.phosphor   #55FFA6   keyboard owner — 1 per screen, always

### Neon — cyan is STRUCTURE
flux.deep        #082F31
flux.core        #128C86   grid, corners, labels
flux             #22E0D0   glyph, numbering, second dimension

### States — no green, no cyan
state.working    #6C8076   ▸ working
state.read       #7DB7E8   ⬤ responded
state.block      #FFB454   ⏵ approve
state.dead       #FF3B47   ✖ crashed
state.off        #3E534E   ○ stopped
state.orphan     #C77DFF   ⚠ orphaned

### ANSI 16
bg #070B0C · fg #BFD1C6 · cursor #55FFA6 · cursor_text #030507
selection_bg #121C1F · selection_fg #E8F4EC
0  #070B0C   1  #C22F38   2  #1F7A4C   3  #C9A227
4  #3E7FA8   5  #8B4FC4   6  #128C86   7  #BFD1C6
8  #3E534E   9  #FF3B47   10 #55FFA6   11 #FFB454
12 #7DB7E8   13 #C77DFF   14 #22E0D0   15 #E8F4EC

## INVIOLABLE RULES

1. Green is never a state color. Green means only "your keyboard is here".
2. Cyan is never a state color. Cyan means only structure/grid.
3. #55FFA6 appears at most once per screen.
4. No glow, no scanline, no chromatic aberration inside the terminal —
   those effects exist only on the brand surface (README, site, banner).
5. Urgency is filled area, not hue: `approve` always renders as a
   solid inverted bar spanning the full line; `responded` is just a glyph.
6. Everything must stay distinguishable with NO_COLOR=1 and on a 16-color terminal.
7. No font ligatures. No emoji. No rounded corners.

## SYMBOL — character version (7×5), canonical

```
┌────┐
│┌───┼┐
││ ▓ ││
└┼───┘│
 └────┘
```

Coloring: back square (inner, offset) in `flux` #22E0D0,
front square in `brand.core` #1F7A4C, `▓` in `brand.phosphor` #55FFA6.
Single-character glyph: ⧉ (U+29C9).

## DELIVERABLES

Generate each file below, complete and valid. One file per block, with the
path at the top. Don't summarize, don't write "and so on".

### A — Terminal colorschemes
1.  `windows-terminal.json`   — `schemes` array fragment, name "Tesseract Neon"
2.  `wezterm.toml`
3.  `alacritty.toml`
4.  `kitty.conf`
5.  `ghostty`                 — theme config file
6.  `tesseract-neon.yaml`     — base16/base24 scheme (tinted-theming),
                                mapping base00–base0F from the palette above.
                                Explain each base's choice in a comment.

### B — Everyday tools
7.  `tmux.conf`               — status bar: green only on the active window, cyan for structure
8.  `starship.toml`           — prompt with `⧉`, green only on the ownership symbol
9.  `fzf.env`                 — FZF_DEFAULT_OPTS variable with the colors
10. `bat.tmTheme`             — theme for bat/delta
11. `delta.gitconfig`         — git's [delta] section
12. `nvim/tesseract.lua`      — minimal but complete Neovim colorscheme:
                                Normal, Comment, String, Function, Keyword,
                                Type, Constant, DiagnosticError/Warn/Info/Hint,
                                CursorLine, Visual, StatusLine, WinSeparator,
                                DiffAdd/Change/Delete, Search, Pmenu
13. `eza-ls-colors.sh`        — LS_COLORS/EZA_COLORS

### C — Tesseract's own theme
14. Theme file in the project's stack, with the 20 tokens as named
    constants (never loose hex in UI code), plus:
    - state → color + glyph + style (normal / inverted) mapping
    - two modes: NAVIGATE (single border) and TYPE (double border +
      inverted `▓ TYPE ▓` badge)
    - fallback for 16 colors and for NO_COLOR=1

### D — README
15. Complete `README.md`, in English, with:
    - ASCII banner at the top, inside a code block:
      ```
         ┌────┐
         │┌───┼┐    T E S S E R A C T
         ││ ▓ ││    the mosaic never falls apart
         └┼───┘│
          └────┘    ts 0.1.0 // MIT
      ```
    - Badges generated on shields.io with `style=flat-square`,
      `color=55FFA6`, and `labelColor=070B0C`: license, version, platform
    - Table of the state alphabet (signal, state, what to do)
    - "The two modes" section explaining NAVIGATE × TYPE
    - Installation, shortcuts, theme configuration
    - Rule: the banner needs to read on GitHub's LIGHT theme too —
      don't use anything that depends on color.
16. `logo.svg` — two 100×100 squares, stroke 8, the back one offset +24/+24
    in `#22E0D0`, the front one in `#55FFA6`, an 18×18 tessera in `#E8F4EC`.
    No gradient, no shadow, no border radius. viewBox "0 0 136 136".
17. `logo-mono.svg` — same geometry, stroke 11, single color `currentColor`,
    WITHOUT the tessera (favicon/16px version).

### E — Verification
18. `scripts/check-theme.sh` — script that:
    - fails if any green or cyan hex from the palette appears in the state map
    - fails if hardcoded hex exists outside the theme file
    - prints all colors in ANSI blocks for visual inspection

## RESPONSE FORMAT

One file at a time, in order 1→18, each in a code block with the full
path as a comment on the first line. No preamble, no final summary.
If any format requires a decision not covered by the palette, choose the
most conservative option and record the decision in a comment in the file itself.
````

---

## How to use

| Situation | What to do |
|---|---|
| Want everything at once | Paste the whole block |
| Only the terminal colorscheme | Delete sections B, C, D, E from DELIVERABLES |
| Want to distribute publicly | Ask for item 6 (base16) first — one YAML yields ~50 applications via `tinted-builder` |
| Running in a small context | Split by section: A, then B, then C, then D |
