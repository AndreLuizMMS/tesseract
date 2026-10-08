# Architecture

## How this doesn't turn into a Frankenstein

Three hard rules, and each one has a test that breaks if it's violated:

1. **A cell type is a sealed piece.** A type declares how it's born, how it draws itself,
   what it does with a key, and what states it has. Adding a type means writing a file in
   `internal/cell/` and one line in the registry — the mosaic, the list, the shortcuts and
   the engine aren't touched. The tabbed session is just another type, made from the others.
2. **A new feature doesn't get a new key.** The keymap lives in a single file. A test walks
   the whole map and fails if the same key has two meanings in the same mode, if any key is
   left without help text, or if any letter moves around the grid.
3. **The screen doesn't decide anything.** It draws what the engine tells it and returns
   keys. A test feeds the list and the mosaic the same state and requires both to show the
   same projects, cells and markers.

## How the code is split

```
cmd/tess/              the ts command: starts the engine or connects to it
internal/
  engine/              state, projects, lifecycle, persistence, warnings
    history/           recording, rotation and search per cell
  cell/                one file per type, over a single contract
  docker/              reads the compose file and acts on the stack, never destructively
  protocol/            engine ↔ screen contract, JSON per line over a unix socket
  keyboard/            the keymap, in a single file
  screen/              mosaic, list, panel, form
  theme/               the palette and the laws of color
mods/                  what Claude Code loads inside a cell (one folder per mod)
systemd/               the user unit that keeps the engine up
```

The engine keeps **the internal screen of each cell in memory** — it already knows what's
written in each one, without asking anyone every frame. That's what keeps the mosaic fluid
with fifteen live cells.

## How the engine talks to Claude Code

Tesseract runs *outside* the agent, so by itself it can only read the cell's screen and
type into it. For Claude Code there is a second way in: a **mod** (`mods/tesseract-link`),
a plugin whose code runs inside the session and sees the real events. The engine starts
every Claude Code cell with `--plugin-dir` for each `mods/tesseract-*` folder it finds, and
the mod only acts when it finds itself inside a cell.

```
Claude Code session                        engine
  turn started / ended   ── ts report ──▶  cell state: working, answered
  permission dialog      ── ts report ──▶  approve + "Bash: pnpm migrate" in the notice
  session id changed     ── ts report ──▶  saved, so a reboot resumes the right conversation
  usage measured         ── ts report ──▶  quota badge
  $.prompt.submit        ◀── ts link ────  prompt sent from the mosaic
```

- **Mod → engine** is `ts report '<json>'`, one short-lived process per event, over the
  same socket and envelope the screen uses. A mod has no socket of its own and writing a
  file isn't atomic; running a program is what it can do. The cell is taken from the
  environment the engine gave the process (`TESSERACT_CELL_ID`, `TESSERACT_TAB`), never
  from the report, and every field is checked on arrival — the session id has to look
  like one, because it becomes a file name.
- **Engine → mod** is `ts link`, one process that lives as long as the session and prints
  a JSON line per prompt. No polling, and the connection doubles as the mod's heartbeat:
  when it drops, the engine knows nobody is speaking for that cell any more.
  A line written to that connection isn't a prompt delivered: each one carries an id,
  and the engine waits two seconds for the mod's receipt (`ts report` with
  `kind: "delivered"`). A receipt marked `refused` — the conversation was sent to Claude
  Code's background and the cell shows its list of agents — becomes an error on the
  footer; no receipt at all, and the text is typed into the cell's terminal the way it is
  for an agent with no mod. A prompt never disappears in silence.
- **One cell, one voice.** Claude Code's background sessions are born from the cell's
  process and inherit its environment and mods. They aren't what the cell shows, so they
  stay mute: the mod checks `CLAUDE_CODE_SESSION_KIND`, and `ts report` / `ts link`
  refuse one. When more than one link is attached to a cell anyway (a mod reloading), the
  newest owns it and the one before takes over when it drops.
- **The mod's word beats the screen, and the screen is still read.** From a link's first
  report on, the cell's turn state is what the mod reported. With no link — an older
  Claude Code, a mod that failed to load, a cursor cell, the folder-trust question that
  shows up before any mod loads — the screen reading in `internal/cell/turn.go` decides,
  exactly as it did before mods existed. A link that has attached but not spoken yet
  doesn't silence the screen either: the mod knows nothing about a question that was
  answered before it loaded, and freezing the state at attach left the cell on "approve"
  until the first turn. A mod may drop and attach again as many times as it likes. The
  usage badge works the same way: the mod's number
  first, the statusline file when there is none.
- **The screen reading knows the skin's words too.** With `mods/tesseract-skin` loaded,
  Claude Code's spinner becomes `▸ WORKING · …` and a blocked tool call becomes the
  `⏵ APPROVE` bar, and none of Claude Code's own texts are left to find. Only a line that
  *opens* with the marker counts — the skin's band above the prompt carries the turn's
  state too, for the whole turn. And "approve" is a question on screen, nothing else: when
  it leaves the screen the cell stops asking, with or without a mod to say so.
- **One thing the mod can't see** is the moment the user answers a permission dialog: the
  next event only comes when the approved tool finishes, which can take minutes. The cell
  clears "approve" itself when enter, esc or a digit goes into it.

The theme is not the mod's job. Setting it from inside the session writes the user's
global settings; the engine passes `--settings '{"theme":"dark-ansi"}'` on the command
line instead, which holds for that cell's session and nothing else.

## Stack

Go 1.25 and six dependencies, not one more:

| Use | Module |
|---|---|
| PTY | `github.com/creack/pty` |
| Terminal emulation | `github.com/charmbracelet/x/vt` |
| Terminal interface | `charm.land/bubbletea/v2` |
| Styling | `charm.land/lipgloss/v2` |
| Rendered markdown | `charm.land/glamour/v2` |
| File changed | `github.com/fsnotify/fsnotify` |

Docker goes through `docker compose`, notification through `powershell.exe`, communication
over a unix socket with JSON per line, state in a file, tests with the standard library. No
SDK, no gRPC, no database, no test framework.

## Develop

```bash
make gate      # build + vet + tests, the gate for everything
make build     # compiles ./ts right here
make instalar  # installs the command and the service
```

The tests spin up real shells, real Docker stacks and the screen inside a real
pseudo-terminal — the ones that depend on Docker skip themselves automatically when it
isn't available.
