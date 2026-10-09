# Theme

Color here isn't decoration, it's grammar. Four rules, and none of them is aesthetic:

- **A project has a hue of its own.** It paints the project's name and the line of its
  cells — dim on every cell, vivid on the one the keys act on — and it never means a state.
- **A state is a glyph in a state color.** It never paints a line. Urgency is filled area,
  not hue: `approve` is a solid bar across the cell's top line, and it is the **only** solid
  area on the screen.
- **Phosphor green is where the typing lands.** The word `TYPE` on the cell that holds the
  keyboard, the caret of a field, the cursor. At most once per screen.
- **Everything else is the neutral ladder.** Text, keys, dialogs. Cyan is left to the brand's
  own mark and to the markdown reader's typography; it structures nothing.

Beyond that: no glow, no gradient, no scanline, no ligature, no emoji. Every box is one thin
line with rounded corners (`╭╮╰╯`), whoever holds the keyboard.

This page describes what the product draws. Where that parts ways with the brand sheet
(`style/tesseract-brand-identity.html`, *v2.0 — neon*) is listed at the end, in
[Where the product diverges from the brand sheet](#where-the-product-diverges-from-the-brand-sheet).

## The screen at rest

```
  doxar-api                                                    api-legado
╭ claude  cursor  bash  md  refatora auth ── Moved the check ⬤ ╮ ╭ md  spec-m7.md ────────────────────────────────── ○ ╮
│Moved the token check                                     │ │# Module 7                                               │
│to the guard.                                             │ │                                                         │
╰──────────────────────────────────────────────────────────╯ │                                                         │
╭ claude  cursor  bash  md  testes ───────────── ctx 86% ▸ ╮ │                                                         │
│$ go test ./...                                           │ │                                                         │
╰──────────────────────────────────────────────────────────╯ │                                                         │
  cortz-web  ✖ docker                                        │                                                         │
╭ claude  cursor  bash  md  fix nav ──────────── ⏵ APPROVE ╮ │                                                         │
│can I touch the Header?                                   │ │                                                         │
╰──────────────────────────────────────────────────────────╯ ╰─────────────────────────────────────────────────────────╯
 ?                                                                                                 ⬤ 1  ⏵ 1   quota 21%
```

Every piece of information is on the screen once:

| Piece | Where, and only there |
|---|---|
| Project | its name, in its hue, over its cells. No number, no path, no rule, no counter |
| A stack with a fallen service | `✖ docker` beside the project's name, in the crashed color — gone when the stack is fine |
| Cell | its top line: the tabs (the one showing is heavy, the others stay a step down), then the name |
| State | the glyph that ends the top line — the same column on every cell. No word |
| The answer in one line | before the glyph, when the agent's mod wrote one and it fits |
| A conversation about to run out of window | `ctx 86%` before the glyph, from 80% up |
| Who's calling, across the grid | `⬤ 2  ⏵ 1` on the status line |
| Quota | `quota 21%` on the status line; from 80% up it turns amber and says when it rolls over |
| Cells that fell off the screen | `+3 off screen · v lists them` on the status line, while the application has the keyboard (in TYPE `v` is the cell's) |

What left the screen: the wordmark stripe and the mode's name at the top, the footer's row of
shortcuts, the project's number, path, rule, per-project counters and `docker 4/5`, the
raised band behind a cell's header, every state's word but `APPROVE`, the double line and
the `▓ TYPE ▓` badge. The keys didn't go anywhere — `?` lists all of them, and the glyphs'
words with them.

The **status line** is the last line of the screen. At rest it is one key: `?`. It speaks
only while there's something to say — an error (`✖ …`), a notice, what a blocked cell is
waiting for and the keys that settle it, the markdown an agent just wrote (`m reads
plan.md`). Under a dialog it says nothing: every dialog ends in its own keys.

## The palette

The whole palette lives in **a single file**, `internal/theme/theme.go`. No other Go file
writes a color: whoever draws asks for the token by name. The mod that draws inside a Claude
Code session keeps its own copy in `mods/tesseract-skin/hooks/tokens.ts`.

### Neutral ladder

| Token | Hex | Where the screen uses it |
|---|---|---|
| `bg.void` | `#030507` | the whole screen while a cell holds the keyboard |
| `bg.base` | `#070B0C` | the whole screen otherwise; text on the `approve` bar |
| `bg.surface` | `#0C1315` | the body of the focused cell and of a dialog — one step above the screen |
| `bg.raised` | `#121C1F` | the selected row of a list; text marked with the mouse |
| `line.dim` | `#16282A` | every line but the owner's while a cell holds the keyboard |
| `fg.faint` | `#3E534E` | what is off: the tabs not showing on an unfocused cell, everything else in TYPE |
| `fg.muted` | `#6C8076` | secondary text: an unfocused cell's content, what a key does, the quota |
| `fg.default` | `#BFD1C6` | primary text; a key's name; the line of a dialog |
| `fg.bright` | `#E8F4EC` | titles: the focused cell's name, a dialog's, the bar of a selected row |

Only what has the focus paints a background of its own. An unfocused cell sits straight on
the screen.

### Project hues

Six, each with a vivid strength (the focused cell's line, the focused project's name) and a
dim one (every other cell of that project).

| Hue | Vivid | Dim | 256 colors | ANSI 16 |
|---|---|---|---|---|
| teal | `#49D4CF` | `#2B807D` | 80 / 30 | 6 |
| pink | `#FC88CB` | `#A85487` | 212 / 132 | 1 |
| lime | `#94DA62` | `#5B8240` | 113 / 65 | 2 |
| indigo | `#8F96FF` | `#646AB9` | 105 / 61 | 4 |
| lemon | `#E1E561` | `#7B7D3B` | 185 / 101 | 3 |
| purple | `#DA97FC` | `#985FB4` | 177 / 97 | 5 |

- **Why these six.** Each sits at least 28° of hue away from every state color and from the
  phosphor, and they are handed out in an order that puts the most distant ones first. Six,
  because ANSI 16 has six chromatic slots: on a 16-color terminal every hue lands on a slot
  of its own.
- **Who gets which.** The engine gives a new project the hue the fewest projects are
  wearing, and never the one of the project right before it. The number is saved with the
  project (`state.json`), so a project keeps its hue across restarts. With more than six
  projects hues repeat; neighbors still differ.
- **What a hue paints.** The project's name and the line of its cells. Nothing else: not a
  glyph, not a word of the header, not a dialog.
- A test (`TestProjectHuesStayApart`) keeps every hue distinct from the others, from the
  state colors and from the neutral ladder — as written, rounded to 256 colors, and on
  ANSI 16.

### States

Shape comes before color: every state has a glyph of its own, and the glyph is all a cell
wears.

| Glyph | State | Color | What it means |
|---|---|---|---|
| `▸` | working | `#6C8076` | deliberately dim |
| `⬤` | responded | `#7DB7E8` | something to read |
| `⏵ APPROVE` | approve | `#FFB454` | blocked on you — **the one solid bar**, across the cell's whole top line, blinking every 2 s; the dim frame drops the fill to half strength (`#835F30`) and stays a bar |
| `✖` | crashed | `#FF3B47` | the process died on its own |
| `○` | stopped | `#3E534E` | no process, cell preserved |
| `◐` | interrupted | `#3E534E` | a turn stopped halfway — by you |
| `◌` | background | `#3E534E` | the conversation isn't on this cell's screen |
| `⚠` | orphaned | `#FFB454` | the project's directory vanished — a warning, in the warning's amber, never a bar |
| *(none)* | idle | — | an agent at its prompt has nothing to say |

The four that say "nothing is running here" are one family, the circle: full when there's
an answer, empty, half, dotted. `approve` is the only state written out, because it is the
only one that is an area. The words of all the others are in the help (`?`, under STATES),
in the notices, and in `ts status`, which prints the glyph and its word (`⬤ responded    session 1`).

Amber is the color of whatever needs you: the `approve` bar, an orphaned project, a window
or a quota about to run out, a kill that takes a project along. Only `approve` fills area.

### Where the typing lands

| Token | Hex | Where |
|---|---|---|
| `brand.phosphor` | `#55FFA6` | the word `TYPE` on the cell that holds the keyboard; the caret `▏` of a field; the terminal's cursor inside that cell |

### The brand's own mark

| Token | Hex | Where |
|---|---|---|
| `flux` | `#22E0D0` | the symbol's back square; in the markdown reader, headings and links |
| `flux.core` | `#128C86` | in the markdown reader, sections, inline code, keywords |
| `brand.core` | `#1F7A4C` | the symbol's front square |

The guard of the palette is executable:

```bash
./scripts/check-theme.sh
```

It prints the whole palette in ANSI blocks for a visual check, and **fails** if a state uses
a project hue, a green or a cyan, or if anyone writes a color outside the theme file — a hex,
or a color escape code typed by hand.

## Who has the keyboard

The stroke of a box never changes. A cell that takes the keyboard is said three ways at
once, and two of them need no color:

| | The application has the keyboard | A cell has it (TYPE) |
|---|---|---|
| The focused cell | vivid line, body on `bg.surface` | the same, plus the word ` TYPE ` at the end of its bottom line, in phosphor |
| Every other cell | its project's hue, dim | no hue: `line.dim` and faint text, `ctx 86%` included — the bar of a blocked cell is the one thing that doesn't sink |
| Other projects' names | their hue, dim | faint; a fallen stack's `✖ docker` stays lit |
| Screen background | `bg.base` | `bg.void` |
| Status line | `?` | `ctrl+l` — the one key that isn't the cell's |

```
╭ claude  cursor  bash  md  testes ───────────── ctx 86% ▸ ╮
│$ go test ./...                                           │
╰─────────────────────────────────────────────────── TYPE ─╯
```

The word is a word: heavy, never a filled badge. Dialogs follow the same idea — the box that
holds the keyboard is the brightest neutral line on the screen, and the field being written
has the phosphor caret.

## Color profiles

The theme has three profiles and picks on its own:

| Profile | When | What happens |
|---|---|---|
| Full color | the default | the hex codes go out as they are; a 256-color terminal gets them rounded by the renderer, to the indexes in the table above |
| 16 colors | `TERM` contains `16color` | each token falls on an index of the ANSI table |
| No color | `NO_COLOR` set, or `TERM=dumb` | bold, faint, underline and reverse video only |

**On 16 colors** the table splits in two: the plain indexes 1 to 6 are the six project hues,
the bright ones are state (9 crashed, 11 approve, 12 responded) and the phosphor (10). Both
strengths of a hue are one index there, so focus is the weight of the line: heavy on the
focused cell, plain on the others. The raised and surface backgrounds have no index and fall
flat.

**With no color** nothing is lost, because nothing depended on hue alone:

- focus — the focused cell's line and name are heavy; every other cell is faint (the
  terminal's own attribute), text and line;
- TYPE — the word on the owner's bottom line, the status line's `ctrl+l`, and everything
  else faint;
- state — the glyph;
- `approve` — the **only** thing in reverse video, and it doesn't blink: turning it off now
  and then would remove the filled area;
- marked text — underlined.

The terminal's own background, text and cursor are only imposed in full color (`bg.base`,
`fg.default`, and a phosphor cursor while a cell holds the keyboard — all three handed back
on exit, the cursor as soon as `ctrl+l` returns the keyboard). On 16 colors the terminal's
palette is left alone; with no color nothing is imposed at all.

| Variable | What it does |
|---|---|
| `NO_COLOR` | strips all color; leaves weight, faint, underline, reverse video and the line |
| `TESSERACT_NO_BLINK` | stops the `approve` bar's blink, keeping the bar |
| `TESSERACT_NO_OPENING` | skips the startup animation even on a first run: the static banner, then the grid |

## The other surfaces

All of them are drawn with the same pieces.

| Surface | How it reads |
|---|---|
| Full screen (`o`) | the focused cell, alone, with the status line under it |
| List (`v`) | on the left a column of text with no frame: each project's name in its hue and, under it, a glyph and a name per cell; the focused row is raised, with a bar in the project's hue, and its glyph keeps the state's color. A blocked cell is its amber glyph there, never a bar: the solid area is the cell's own top line, on the right, where the cell itself is drawn by the same hand that draws it on the grid |
| Dialogs — new cell, prompt, search, kill, Docker (`d`), IDE (`ctrl+e`), help (`?`) | one rounded box in the neutral line over `bg.surface`, the title in the top line, and its keys as its last lines, in their short form (`esc close`). The rows a dialog sits on are cleared from edge to edge — no slice of a cell is left beside it — and the grid stays whole above and below. A selected row is raised with a bar on its left; the labels of a table are faint |
| Docker panel | a service wears a cell's glyph: `▸` up, `✖` fell, `○` stopped. While open it reads the stack again every 3 s |
| Markdown tab | the list and the reader's header are the neutral ladder; the page itself is typeset in the two cyans (headings, links, code) |
| Empty grid | the brand's symbol in the middle, and the key that creates the first cell |
| Inside a Claude Code cell | `tesseract-skin` uses the same glyphs and no state word: `▸ thinking · 12s`, `⬤ 42s`, `◐ 4s`. The hue and the phosphor never show up in there — they belong to the cell's line |

## Where the brand shows up

| Place | Form |
|---|---|
| Startup | the symbol drawing itself, stroke by stroke, on launch |
| Empty grid | the 7×5 symbol at the center, with the key that creates the first cell |
| Window title | `⧉ ts — project/cell`, following the focus |
| `ts status`, `ts help` | the `⧉` glyph opens the line |

The character version is the canonical one, 7×5: the back square in cyan from the top left,
the front one in green, the tessera in phosphor. It keeps the brand's square corners — it is
a drawing of the mark, not a box of the interface.

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

On a grid with nothing to bring back — a first run — the tally says so instead of counting
zeros: `no cells to recover // the first one opens here // 41ms`.

The drawing lasts about three seconds, so it **only runs over an empty grid** — the first
run, or a grid you emptied. With cells to restore the banner shows up already drawn and
the mosaic opens right behind it. The stopwatch at the end of the tally only counts the
engine, never the animation.

What moves is the **order in which things come into existence**, not their texture. Without
a real terminal (output to a file, a pipe), the startup becomes the static block, written
once. In a terminal narrower than the block (42 columns) the static block stacks: the
signature goes under the symbol and the tally breaks into one figure per line. The banner is
written outside the screen's program, and it goes through the same color downgrade the grid
gets.

## Tone of voice

The microcopy is dry and ends in a key: `No cells yet. n creates the first one.` It never
apologizes, never celebrates and never plays a character. An empty state says what is empty
and which key changes that; an error says what failed, after the `✖` that tells it apart
with no color.

## Where the product diverges from the brand sheet

`style/tesseract-brand-identity.html` (v2.0) was not changed. These are the places where the
screen no longer does what it says, so the sheet can be brought up to date — or the product
brought back:

| Brand sheet v2.0 | Product now |
|---|---|
| One vivid color: phosphor green, for keyboard ownership | Six project hues are vivid too; the phosphor is narrowed to *where the typing lands* (the `TYPE` word, caret, cursor) and no longer paints a border |
| Cyan is structure: rule, corners, labels, numbering, keys | Structure is the project's hue (lines) and the neutral ladder (labels, keys). Cyan is only the brand's mark and the markdown reader's typography |
| Focused cell: live green line (`brand.live`); focused project: `brand.core` | Focused cell: its project's vivid hue. `brand.live` and `brand.deep` are unused on the screen |
| TYPE: double line (`╔═╗`), `▓ TYPE ▓` inverted badge in the top-right corner | TYPE: same thin line, the word ` TYPE ` on the owner's bottom line, everything else dimmed |
| No rounded corner inside the terminal | Every box has rounded corners; only the symbol keeps square ones |
| Each state is a glyph **and its word** (`▸ WORKING`, `⬤ RESPONDED`, …) | A glyph alone; only `⏵ APPROVE` is written out. The words live in the help, the notices and `ts status` |
| `○` for stopped, interrupted and background, told apart by the word | Three glyphs: `○`, `◐`, `◌` |
| `⚠ orphaned` in violet `#C77DFF` | `⚠` in the warning amber `#FFB454`. Violet left the state table so it could be a project hue's neighborhood; `#C77DFF` remains ANSI 13 in the terminal themes |
| Header: the wordmark `T E S S E R A C T` on a cyan rule, and a status line under it | No header. One status line, at the bottom |
| Footer: a row of shortcuts | `?` — the help lists every key |
| Project divider: number, name in capitals, path, rule, counters, `docker 4/5` | The name, as it is on disk, in the project's hue |
| A cell's header is a raised band (`bg.raised`); every cell sits on `bg.surface` | No band; only the focused cell sits on `bg.surface` |
| ANSI 16: indexes 1–6 have no role (4 of them "spare") | Indexes 1–6 are the six project hues |

Unchanged: the dark base and its ten shades, the state colors for working, responded,
approve, crashed and stopped, `approve` as the only solid area and its 2 s blink, the
symbol, the startup, the tone of voice, and the terminal themes in `themes/`.

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
`#BFD1C6`, `cursor` `#55FFA6`, `cursor_text` `#030507`, `selection` `#121C1F`. None of them
changed with the project hues: the ANSI table is the same, the product only gave its plain
half a job.

**The agent inside the cell.** Claude Code paints with its own palette and clashes with the
grid — orange on the badge, pink on the logo. The fix is one line: `/config` → theme →
**`dark-ansi`**. In that theme it draws only with the 16 ANSI colors, which are Tesseract
Neon's. Details in [`themes/claude-code.md`](../themes/claude-code.md).

The brand in vector form is at `themes/logo.svg` (colored) and `themes/logo-mono.svg`
(single stroke in `currentColor`, for favicon and 16px). The details of each file are in
[`themes/README.md`](../themes/README.md).
