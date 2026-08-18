# Claude Code theme inside Tesseract

## The problem

Claude Code paints with its own 24-bit palette — orange in the badge, pink
in the logo, blue and magenta in the context bar. None of that knows about
Tesseract Neon, so inside the cell it clashes with the grid: colors the
panel reserved for meaning (orange means "approve", green means "your
keyboard is here") show up in the middle of the conversation without
meaning anything.

Tesseract has no way to censor the agent's color — the cell is a real
terminal, and what the agent writes into it is its own. The one that fixes
this is Claude Code itself, and it already knows how.

## The fix

Claude Code has a theme that **uses no color of its own**: it draws only
with the terminal's 16 ANSI colors. Since Tesseract Neon defines exactly
those 16 colors, Claude Code starts speaking the same language as the grid.

```
Claude Code theme:  dark-ansi
```

To switch, inside any Claude Code session:

```
/config
```

and choose **dark-ansi** in the theme field. It applies to every session,
including ones already running in cells — Claude Code re-reads the theme
immediately.

> Don't edit `~/.claude.json` by hand for this. Every live session rewrites
> that file on exit, and your edit goes away with it. `/config` writes
> through the right path.

## What each color becomes

With `dark-ansi`, Claude Code starts asking for color by index, and the
index is resolved by your terminal emulator's palette. Install one of the
files from `themes/` (`windows-terminal.json`, `kitty.conf`, whichever is
yours) and the result is this:

| What Claude Code paints | ANSI index | Color in Tesseract Neon |
|---|---|---|
| normal text | 7 | `#BFD1C6` fg.default |
| dimmed text, hints | 8 | `#3E534E` fg.faint |
| error, removed diff | 1 / 9 | `#C22F38` / `#FF3B47` state.dead |
| warning, pending permission | 3 / 11 | `#C9A227` / `#FFB454` state.block |
| success, added diff | 2 / 10 | `#1F7A4C` / `#55FFA6` brand |
| link, file reference | 4 / 12 | `#3E7FA8` / `#7DB7E8` state.read |
| highlight, command | 6 / 14 | `#128C86` / `#22E0D0` flux |
| title, emphasis | 15 | `#E8F4EC` fg.bright |

## Why not a theme with its own hex

Claude Code doesn't accept an arbitrary palette: the theme is a choice among
a handful of ready-made options. `dark-ansi` is the only one that hands
color control back to the terminal — and handing control back to the
terminal is exactly what we want, because the terminal here is Tesseract.

It's better than a theme with fixed hex, too: when you swap Tesseract's
palette, Claude Code switches along with it, with nobody reconfiguring
anything.

## The two steps, together

```sh
# 1. the terminal emulator speaks Tesseract Neon
#    (pick the file for your terminal in themes/)

# 2. Claude Code stops having its own color
/config   →   theme   →   dark-ansi
```

The grid's `⏵ APPROVE` remains the only meaningful orange, and phosphor
green keeps appearing once per screen.
