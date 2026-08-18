```
     ┌────┐
     │┌───┼┐    T E S S E R A C T
     ││ ▓ ││    the mosaic doesn't come apart
     └┼───┘│
      └────┘    ts 0.1.0 // MIT
```

[![license](https://img.shields.io/badge/licen%C3%A7a-MIT-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6)](../LICENSE)
[![version](https://img.shields.io/badge/vers%C3%A3o-0.1.0-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6)](https://github.com/andreluiz/tesseract/releases)
[![platform](https://img.shields.io/badge/plataforma-Linux%20%7C%20WSL%20%7C%20macOS-55FFA6?style=flat-square&labelColor=070B0C&color=55FFA6)](#installation)

> The banner above is made only of lines and shading — nothing in it depends
> on color. It reads the same in GitHub's dark and light theme, in `cat`, in
> `less`, and in a terminal with `NO_COLOR=1`. That's the rule: **if it
> disappears when color disappears, it doesn't go in**.

**Tesseract** is a terminal panel where several AI agents run side by side in
a grid. One command, `ts`, and the whole screen is your mosaic: each cell is
a live agent, each project is a lane, and the engine keeps running when you
close the screen.

---

## The alphabet of states

Every cell is in exactly one state, and the state always has **three**
signals: a glyph, a color and a shape. Take away the color and the glyph
remains. Take away the color and the glyph and the shape remains — only the
state that **blocks** takes up the whole line.

| Signal | State | What to do |
|---|---|---|
| `▸ WORKING` | live process producing | nothing — let it work |
| `⬤ REPLIED` | handed back the turn, has text waiting | read when you can; nothing is blocked |
| `⏵ APPROVE` | stuck on a question and **not moving** | answer: the work is stopped on this |
| `✖ DIED` | the process died on its own | `r` brings it back up |
| `○ STOPPED` | no process, cell preserved | `r` resumes where it left off |
| `⚠ ORPHAN` | the project's directory vanished from disk | recreate the path or kill the cell |

**Replied ≠ approve.** That's the distinction that makes the alarm worth
something: an agent stuck on a question blocks the work; an agent that
finished its turn just has something to read.

That's why `⏵ APPROVE` is the **only** line that appears as a solid inverted
bar, taking up the full width: urgency here is filled area, not hue. The
other five states are a glyph and a label.

And **no state color is ever green or cyan** — the two are already owned:

- **green is keyboard ownership.** Phosphor green `#55FFA6` appears at most
  once per screen, in the cell that has your keyboard. Nothing else.
- **cyan is structure.** Grid, corners, numbering, labels. Never a state.

---

## The two modes

There are never two keyboard owners at the same time. That rule is what makes
shortcut collisions structurally impossible.

### NAVIGATE — the keyboard belongs to the app

The default. Every key is a command: arrows move through the grid, letters
act on the focused cell. Cell borders are **simple**, the background is the
default background, and the bottom bar shows the shortcuts.

### TYPE — the keyboard belongs to the cell

`↵` enters. From there **every key goes to the agent, no exceptions**
— not `q`, not `D`, not `tab`, not the arrows. Only `ctrl-l` returns the
keyboard to the app.

The mode is impossible to mistake, because it changes four things at once:

1. the screen background darkens;
2. the focused cell's border becomes **double**;
3. the `▓ TYPE ▓` badge appears inverted;
4. the cell holding the keyboard turns **phosphor green** — and it's the
   only phosphor green on screen.

With `NO_COLOR=1` items 1 and 4 disappear, and items 2 and 3 remain: double
border and inverted badge don't depend on color at all.

---

## Installation

```sh
git clone https://github.com/andreluiz/tesseract
cd tesseract
./install.sh
```

Or building directly:

```sh
go install github.com/andreluiz/tesseract/cmd/ts@latest
```

Then, it's a single command:

```sh
ts
```

Requirements: Go 1.25+ to build, a terminal with 256-color support for the
full experience, and nothing beyond that — Tesseract reads and writes only
in its own config folder and requires no third-party daemon.

---

## Shortcuts

**Move — arrows only, no letters**

| Key | Action |
|---|---|
| `←` `→` | previous / next cell, crossing projects |
| `↑` `↓` | previous / next project |
| `space` | jump to the next cell that needs attention |
| `1`…`9` | jump straight to project N |
| `tab` | switch the cell's tab (`shift-tab` goes back) |

**Keyboard and screen**

| Key | Action |
|---|---|
| `↵` | enters TYPE mode on the focused cell |
| `ctrl-l` | returns the keyboard to the app |
| `o` | focused cell in full screen |
| `v` | toggles mosaic ↔ list |

**Create, kill, rename**

| Key | Action |
|---|---|
| `n` | create — asks for the project, then the cell |
| `r` | resumes a stopped cell, or brings a dead cell back up |
| `D` | kills the focused cell — always confirms |
| `ctrl-r` | adopts, as the cell name, the name the agent gave the conversation |

**Act and read**

| Key | Action |
|---|---|
| `p` | sends a prompt to the focused cell without entering it |
| `d` | opens the focused project's Docker panel |
| `ctrl-e` | opens the project directory in the configured IDE |
| `/` | searches the focused cell's history |
| `esc` | exits scrolling and closes whatever is open |
| `?` | help |
| `q` | closes the screen — the engine keeps running |

---

## Theme configuration

The whole palette lives in a single file, `internal/tema/tema.go`. No other
file in the project writes hex — whatever draws asks for the token by name
(`tema.BrandPhosphor`, `tema.FluxCore`, `tema.StateBlock`). The
`scripts/check-theme.sh` script fails if anyone breaks that rule, or tries
to use green or cyan as a state color.

### The terminal and the tools

The `themes/` folder ships **Tesseract Neon** ready for the rest of the desk:

| File | For |
|---|---|
| `windows-terminal.json` | fragment of the `schemes` array |
| `wezterm.toml` | WezTerm |
| `alacritty.toml` | Alacritty |
| `kitty.conf` | kitty |
| `ghostty` | Ghostty |
| `tesseract-neon.yaml` | base16/base24 scheme (tinted-theming) |
| `tmux.conf` | status bar |
| `starship.toml` | prompt |
| `fzf.env` | `FZF_DEFAULT_OPTS` |
| `bat.tmTheme` | bat and delta |
| `delta.gitconfig` | git's `[delta]` section |
| `nvim/tesseract.lua` | Neovim colorscheme |
| `eza-ls-colors.sh` | `LS_COLORS` and `EZA_COLORS` |
| `claude-code.md` | how to make Claude Code stop having its own color inside the cell |

Each of them follows the same rules as the panel: green only where there's
ownership, cyan only where there's structure, state color in neither.

The syntax theme (`bat.tmTheme`) serves both `bat` and `delta`:

```sh
mkdir -p "$(bat --config-dir)/themes"
cp themes/bat.tmTheme "$(bat --config-dir)/themes/Tesseract Neon.tmTheme"
bat cache --build
export BAT_THEME="Tesseract Neon"
```

### Poor and colorless terminals

The theme has three profiles and picks the right one on its own:

| Profile | When | What changes |
|---|---|---|
| 24-bit | `COLORTERM=truecolor` or `TERM=*256color*` | the palette comes out as-is |
| 16 colors | any other `TERM` with color | each token falls back to its ANSI 16 index |
| no color | `NO_COLOR` set, or `TERM=dumb` | only bold, reverse and border |

In all three, the alphabet of states stays legible — because the glyph and
the shape carry the meaning on their own, and color only reinforces it.

### Checking

```sh
./scripts/check-theme.sh
```

The script prints the whole palette in ANSI blocks for visual inspection, and
fails if it finds green or cyan in the state map, or hex written outside the
theme file.

---

## The brand

The symbol is a flattened tesseract: two squares, one behind the other,
offset — with a lit tessera in the middle.

```
┌────┐
│┌───┼┐
││ ▓ ││
└┼───┘│
 └────┘
```

In the character version: the back square is cyan (`#22E0D0`, the second
dimension), the front one is dark green (`#1F7A4C`), and the tessera is
phosphor (`#55FFA6`). In the vector, the order flips — the front square
carries the phosphor and the tessera is white (`#E8F4EC`), because at thin
stroke widths the dark green disappears against a dark background. As a
single character, the symbol is `⧉` (U+29C9).

Vector versions: `themes/logo.svg` (color) and `themes/logo-mono.svg`
(single stroke in `currentColor`, for favicon and 16px).

Glow, scanline and chromatic aberration exist **only** on the brand surface —
README, site, banner. Never inside the terminal.

---

## License

MIT. See [LICENSE](../LICENSE).
