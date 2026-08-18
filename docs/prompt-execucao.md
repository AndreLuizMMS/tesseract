# Implement Tesseract

## Goal

Deliver Tesseract working: a terminal agent instance manager, running on WSL as a service,
with a mosaic of cells per project, a Docker panel and automatic recovery after a WSL
crash.

## Source of truth

`~/tesseract/docs/specs/2026-08-17-tesseract-design.md` — approved spec, 11 sections.
It rules. Where this prompt and the spec disagree, the spec wins. Read it whole before
writing the first line.

Real conflict between the spec and technical reality: stop, report, propose. Don't decide
alone.

## Context

New project, at `~/tesseract`, still empty outside the `docs/` folder.

There's a personal fork at `~/claude-squad-andre` (Go, ~15k lines, fork of
smtg-ai/claude-squad). **No line of it is reused.** It serves as a behavior reference on
three specific points, and only those:

- `session/instance.go` and `app/notify.go` — the heuristic that avoids a false alarm when
  detecting the end of an agent's turn (N reads of work to arm, M of silence to declare it
  over)
- `session/quota.go` — reading the 5h-window usage file
- `README.md` §3.5 — that file's format

Environment verified on this machine:

| Item | State |
|---|---|
| Go | 1.25.8 |
| systemd on WSL | active as pid 1 (`systemd=true` in `/etc/wsl.conf`) |
| Docker Compose | v2.35.1, accepts `--format json` |
| Windows interop | on; `powershell.exe` and `cmd.exe` reachable |
| `wsl-notify-send.exe` | **not installed** — notification goes through `powershell.exe` |
| Terminal | Windows Terminal (`xterm-256color`) |

## Stack — fixed

Go 1.25. Exactly six dependencies, not one more:

| Use | Module |
|---|---|
| PTY | `github.com/creack/pty` |
| Terminal emulation | `github.com/charmbracelet/x/vt` |
| Terminal interface | `github.com/charmbracelet/bubbletea` v2 |
| Styling | `github.com/charmbracelet/lipgloss` v2 |
| Rendered markdown | `github.com/charmbracelet/glamour` |
| File changed | `github.com/fsnotify/fsnotify` |

Everything else is the standard library. Docker via `exec` of `docker compose`, no SDK.
Notification via `exec` of `powershell.exe`, no library. Communication between engine and
screen over a unix socket with JSON per line, no gRPC. State and history in a file, no
database. Tests with stdlib `testing`, no framework.

Need a seventh dependency: stop and propose before adding it.

## Structure — fixed

```
tesseract/
├── cmd/tess/              single entry point: starts the engine or connects to it
├── internal/
│   ├── motor/             state, projects, lifecycle, persistence
│   │   └── historico/     recording, rotation, search
│   ├── celula/            one file per type
│   │   ├── celula.go         the contract: is born, draws, receives keys, has states
│   │   ├── processo.go       shared base for the ones that run something
│   │   ├── claude.go  cursor.go  bash.go  logs.go  md.go
│   ├── docker/            reads the compose file, acts on the stack and the service
│   ├── protocolo/         engine↔screen contract
│   ├── teclado/           the keymap, in a single file
│   └── tela/              mosaico.go  lista.go  docker.go  formulario.go
└── systemd/tesseract.service
```

That structure exists to support three rules from spec §8. They're mandatory:

1. A new cell type is a file in `celula/` plus a line in the registry. `tela/`, `teclado/`
   and `motor/` aren't touched.
2. The keymap lives only in `teclado/`. No file outside it ties a key to an action.
3. `tela/` draws what the engine tells it and returns keys. No business rule lives there.

## How to execute: six gated phases

Inside a phase, iterate on your own: implement, run `go build ./... && go vet ./... &&
go test ./...`, fix, repeat until green. Don't ask the human anything mid-phase.

At the end of each phase: stop, deliver that phase's manual test script, and wait for
feedback. An error reported during manual validation goes back inside the phase — fix it
and hand back the script again. Only move on once the human gives the OK.

---

### Phase 1 — vertical slice

Engine with one `bash` cell: PTY, in-memory terminal emulation, history in a file, unix
socket, and a screen showing that cell full-screen with the two modes.

**Required automated tests**

- `protocolo`: serializing and deserializing every message returns the original value.
- `celula/processo`: starts `bash`, writes `echo tesseract\n`, and the engine's in-memory
  screen comes to contain `tesseract` within 2s.
- `motor/historico`: writing past the size ceiling discards from the start, keeping the
  end; search by term returns the right lines with the line number.
- `motor`: desired state is written atomically; a corrupted file loads what it can,
  preserves the original with a suffix and returns the error.

**Manual validation**

1. `go run ./cmd/tess` — opens with one `bash` cell full-screen, NAVIGATE mode, with the
   title bar and footer lit.
2. `↵` — enters TYPE: bar and footer dim, the `▓ TYPE ▓` badge appears, the cell's border
   thickens.
3. Type `q` and `D` — both letters show up in the shell, nothing happens in the app.
4. `ls -la` + `↵` — the output shows up. `ctrl-l` — back to NAVIGATE, chrome lights up.
5. `q` — the screen closes. `tess status` shows the engine alive with one cell.
6. `go run ./cmd/tess` again — the same cell is there, with the `ls` output still visible,
   and the shell responds.
7. Scroll with the mouse wheel — the history scrolls up; `esc` goes back to live.

---

### Phase 2 — projects and mosaic

Projects as a column, a status strip for the unfocused ones, arrow navigation, a single
creation form, `n` `D` `o` `v`.

**Required automated tests**

- `teclado`: **no key has two meanings** in the same mode; in TYPE mode the only reserved
  key is `ctrl-l`; every key in the map has help text. That test is the mechanical
  guarantee of rule 2 — if it breaks, the rule was violated.
- `tela/mosaico`: with 3 projects and a fixed width of 120 columns, the focused column gets
  reading width and the others become a strip; the rendered output matches a golden file.
  Changing focus changes which column fattens.
- `motor`: killing a project's last cell removes the project from the state; killing a
  non-last cell doesn't.
- `motor`: creating a cell in a nonexistent path or without write permission fails with a
  clear error and doesn't change the state.

**Manual validation**

1. `n` — the form opens with PROJECT filled with the focused project.
2. Type a new path, `tab` completes it and shows how many folders match. Choose `bash`,
   give it a name, `↵`.
3. The second project shows up as a column. The focused column is wide; the other one
   became a strip with vertical initials, cell count and Docker indicator.
4. `←` `→` — the neighboring column fattens and the current one shrinks. `↑` `↓` move
   between cells.
5. Type `j`, `k`, `h`, `l` in NAVIGATE — nothing happens; those letters don't navigate.
6. `o` — the focused cell takes over the screen. `shift` + drag selects text from it only,
   without grabbing the neighbor. `esc` goes back to the mosaic.
7. `D` — asks for confirmation. On a project's last cell, the confirmation warns that the
   project will leave the screen. Confirm: the project disappears, and the directory stays
   intact on disk.

---

### Phase 3 — the other cell types

`claude`, `cursor`, `logs`, `md`. Turn states with the anti-false-alarm heuristic.
`p`, `r`, `R`, `ctrl-r`, `space`, `1`…`9`.

**Required automated tests**

- Turn state: feeding the heuristic a synthetic byte stream, a blinking spinner and a
  moving cursor **don't** trigger `answered`; work followed by silence triggers it; a
  blocking question turns into `approve`, not `answered`.
- `celula/md`: changing the file on disk updates the cell within 1s; a deleted file turns
  into a readable error state, not a panic.
- `celula/logs`: a stopped service leaves the cell `stopped` and it hooks in when the
  service comes up.
- `celula`: every type implements the whole contract — a table test that walks the type
  registry and fails if any type doesn't respond to being born, drawing, receiving a key
  and reporting states.

**Manual validation**

1. Create a `claude` cell in a real project. It comes up and Claude's screen appears.
2. `p` — sends a prompt without entering the cell. Claude starts working; the marker turns
   to working.
3. It finishes: the marker turns to `⬤ ANSWERED`, the alert sounds, the system notification
   comes up with the cell's and the project's name.
4. Ask for something that needs approval: the marker turns to `⏵ APPROVE`, with a color
   different from answered.
5. `space` from another project — jumps straight to the cell that called.
6. `R` — rename the cell. The name changes on screen **and** the conversation inside Claude
   is renamed. `ctrl-r` — the cell adopts the name Claude gave the conversation.
7. Repeat 1, 2 and 6 with `cursor`.
8. Create an `md` cell pointing at a file. Ask Claude to edit that file: the rendered
   markdown updates itself alongside it.
9. Kill Claude's process from outside (`kill`): the cell turns `✖ crashed`, alerts, and
   **doesn't** restart on its own. `r` brings it back up, resuming the conversation.

---

### Phase 4 — Docker

Project panel, actions on the service and the stack, log becomes a cell.

**Required automated tests**

- `docker`: parsing the output of `docker compose ps --format json` from fixtures covering
  a service up, exited, without a port and without a healthcheck.
- `docker`: compose file detection only at the project root, in the order
  `docker-compose.yml`, `docker-compose.yaml`, `compose.yml`, `compose.yaml`; no recursive
  search; a project without compose gets no panel.
- `teclado`: the Docker panel doesn't reuse any key with a different meaning than the
  global map unless it's declared as the panel's own keymap.

**Manual validation**

On one of your projects with a real Compose file:

1. `d` — the panel opens listing the services with state, port, health and uptime.
2. `↑` `↓` pick a service — they don't run any action.
3. `s` stops the chosen service; `u` brings it back up; `r` restarts it. The list reflects
   each change.
4. `U` brings up the whole stack; `S` stops everything; `R` restarts everything.
5. `l` on a service — the panel closes and a `logs` cell for that service is born in the
   mosaic, with the log running.
6. That cell behaves like any other: `D` kills, `R` renames, mouse wheel scrolls, `o` puts
   it full screen.
7. There's no action anywhere in the panel that deletes a volume or brings things down with
   `-v`.

---

### Phase 5 — service and recovery

User systemd unit, reconstitution after a crash, notification from the engine, usage
badge.

**Required automated tests**

- `motor`: killing the engine and bringing it back up from the desired state rebuilds
  projects and cells with the same type, name and position.
- `motor`: a reconstituted `claude` cell **doesn't** fire off any prompt — a test that
  fails if a single byte gets written to the agent's PTY during reconstitution.
- `motor`: Docker doesn't come up by itself on reconstitution.
- `motor/quota`: the badge disappears when the file is stale; changes band above 80%; the
  file's absence doesn't break anything.

**Manual validation**

1. `systemctl --user enable --now tesseract` — the service comes up.
2. Leave 2 projects with 4 cells, 2 of them `claude` with a conversation in progress.
3. Close the screen with `q`. `systemctl --user status tesseract` shows the service alive.
4. Ask something of a Claude via `p`, close the screen, wait for it to finish: **the system
   notification arrives with the screen closed.**
5. On Windows PowerShell: `wsl --shutdown`. Wait. Reopen WSL.
6. `tess` — the same grid, in the same positions, with the same names. The `bash` cells
   have their previous history scrollable. The `claude` ones are **stopped**, with the
   conversation resumed and no new work started.
7. The Docker stack is still where it was — it didn't come up by itself.

---

### Phase 6 — polish

List with a preview, `/` search, `?` help, the five command-line commands, installation.

**Required automated tests**

- `tela`: the list and the mosaic, fed the same state, show the same projects, cells and
  markers. That test is the mechanical guarantee of rule 3.
- `motor/historico`: search returns the correct result in a large file, including a term
  that crosses the rotation boundary.
- `cmd/tess`: every command-line command responds and exits with the right code; `tess`
  with no engine running starts the engine.

**Manual validation**

1. `v` — switches to the list. Same projects, cells and markers as the mosaic, now in
   text, with a live preview of the selected cell on the right.
2. Every key from phases 2 and 3 does **the same thing** in the list: `n`, `D`, `o`, `p`,
   `R`, `r`, `space`, arrows.
3. `/` — searches the focused cell's history and jumps to the match.
4. `?` — the help lists every key, split by mode, with none that doesn't exist.
5. Close and reopen: the chosen screen (list or mosaic) was remembered.
6. `tess status`, `tess novo <dir>`, `tess stop`, `tess reset` — each does what spec §9
   says.
7. Resize the terminal window on each screen: nothing breaks, nothing renders crooked.

---

## Out of scope

Don't build, even if it feels natural:

- A git cell (local tree, diff) and a GitHub PR cell.
- Auto-yes: automatic approval of agent requests, and a daemon that answers for you.
- A written command palette.
- Reordering a cell or project. Order is creation order.
- Editing markdown. The `md` cell only reads.
- Themes, visual configuration, anything about appearance beyond what the spec describes.
- Running outside WSL.
- Reusing code from the fork.

Also not: abstraction for a single use case, an interface with a single implementation,
configuration for a value that never changes, rewriting something that already passed the
gate.

## Constraints

- Interface, error messages and help in Portuguese. Code, identifiers and comments also in
  Portuguese, following the spec's names (`celula`, `motor`, `tela`, `projeto`).
- Commits in pt-br, single subject with no body, only the prefixes `feat:`, `fix:`,
  `refactor:`, no scope, no co-authorship footer.
- `go build ./... && go vet ./... && go test ./...` green before any delivery.
- History ceiling per cell: 5 MB, discarding from the start, configurable.
- If `charmbracelet/x/vt` can't handle some agent, swap it for `hinshun/vt10x` or a custom
  emulation of the needed subset. The swap can't touch anything outside
  `internal/celula/`.
- No destructive Docker action anywhere: no `down -v`, no deleting a volume.
- The human doesn't read code. The end-of-phase report describes observable behavior, not
  implementation.

## Definition of done

The six gated phases with manual approval, `go test ./...` green, and the service
surviving a `wsl --shutdown` with the grid intact and no agent having worked alone.

---

## Decisions made without human confirmation

They stand. If any is wrong, he'll correct it before phase 1 starts.

1. "Recursive until ready" is an autonomous loop inside a phase and a human gate between
   phases — the visual part of a terminal screen has no way to be validated without an eye
   on it.
2. Auto-yes is out of V1 and the `md` cell only reads, per the spec.
3. Notification goes through `powershell.exe`, since `wsl-notify-send.exe` isn't installed.
4. Code and identifiers in Portuguese, following the spec's vocabulary.
5. Phase 1 delivers full screen with one cell before the mosaic, so there's something
   running early instead of two phases with no screen.
6. History ceiling of 5 MB per cell — a number chosen here, not from the spec.
