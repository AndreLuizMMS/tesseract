# Theme and brand

Color here isn't decoration, it's grammar. It has three laws, and none of them is
aesthetic:

- **Green means keyboard ownership.** Never a state. Phosphor green appears at most once per
  screen: on the cell holding your keyboard, and nowhere else.
- **Cyan is structure.** Grid, corners, numbering, labels. Never a state.
- **State doesn't use green or cyan**, and urgency is filled area, not hue.

Beyond that: no glow, no scanline, no ligature, no emoji, no rounded corner inside the
terminal. Those effects only exist on the brand surface — this README, the site, the
banner.

The whole palette lives in **a single file**, `internal/tema/tema.go`. No other file in the
project writes hex: whoever draws asks for the token by name (`tema.BrandPhosphor`,
`tema.FluxCore`, `tema.StateBlock`). The guard of that rule is executable:

```bash
./scripts/check-theme.sh
```

It prints the whole palette in ANSI blocks for a visual check, and **fails** if anyone uses
green or cyan as a state color, or writes hex outside the theme file.

The theme has three profiles and picks on its own: full color, 16 colors, or none
(`NO_COLOR=1` or `TERM=dumb`). In all three the state alphabet stays legible, because the
glyph and the shape carry the meaning and color only reinforces it.

| Variable | What it does |
|---|---|
| `NO_COLOR` | strips all color; leaves bold, reverse video and border |
| `TESSERACT_SEM_PISCA` | stops the `approve` bar's blink, keeping the bar |
| `TESSERACT_SEM_ABERTURA` | skips the startup animation and goes straight to the grid |

### Where the brand shows up

The symbol doesn't only live in the README. Inside the product it has four homes:

| Place | Form |
|---|---|
| Startup | the symbol drawing itself, stroke by stroke, on launch |
| Title bar | the `⧉` glyph, always to the left of the name |
| Empty grid | a 7×5 symbol at the center, with the key that creates the first cell |
| Window title | `⧉ ts — project/cell`, following the focus |

### The startup

The brand doesn't appear ready-made at launch: **it assembles itself**. The back square is
born first, stroke by stroke, with the pen tip lit up in phosphor running along the path.
Then the front square on top. The tessera lights up, the whole symbol bursts in a single
frame and settles. Only then does the name open, letter by letter, and the engine's lines
come in one at a time.

```
   ┌────┐
   │┌───┼┐    T E S S E R A C T
   ││ ▓ ││    the mosaic doesn't fall apart
   └┼───┘│
    └────┘    ts 0.1.0 // MIT

   > session engine: alive
   > 8 cells recovered · 3 projects · same position
   > grid built in 41ms
```

Lasts about a second and a half, and **runs in parallel with the connection** — while the
brand draws itself, the engine is searched for and the grid is rebuilt on the other
execution thread. The `grid built in` stopwatch only counts the engine, never the
animation: it's a number that exists to be true.

None of this is glow, scanline or glitch — the terminal doesn't emit light and the rule
still holds. What moves is the **order in which things come into existence**, not their
texture. Without a real terminal (output to a file, a pipe), the startup becomes the static
block, all at once.

### The same theme across the rest of the desk

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

**The agent inside the cell.** Claude Code paints with its own palette and clashes with the
grid — orange on the badge, pink on the logo. The fix is one line: `/config` → theme →
**`dark-ansi`**. In that theme it draws only with the 16 ANSI colors, which are Tesseract
Neon's. Details in [`themes/claude-code.md`](../themes/claude-code.md).

The brand in vector form is at `themes/logo.svg` (colored) and `themes/logo-mono.svg`
(single stroke in `currentColor`, for favicon and 16px). The details of each file are in
[`themes/README.md`](../themes/README.md).
