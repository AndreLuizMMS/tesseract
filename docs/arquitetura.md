# Architecture

## How this doesn't turn into a Frankenstein

Three hard rules, and each one has a test that breaks if it's violated:

1. **A cell type is a sealed piece.** A type declares how it's born, how it draws itself,
   what it does with a key, and what states it has. Adding a type means writing a file in
   `internal/celula/` and one line in the registry — the mosaic, the list, the shortcuts and
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
  motor/               state, projects, lifecycle, persistence, warnings
    historico/         recording, rotation and search per cell
  celula/              one file per type, over a single contract
  docker/              reads the compose file and acts on the stack, never destructively
  protocolo/           engine ↔ screen contract, JSON per line over a unix socket
  teclado/              the keymap, in a single file
  tela/                mosaic, list, panel, form
systemd/               the user unit that keeps the engine up
```

The engine keeps **the internal screen of each cell in memory** — it already knows what's
written in each one, without asking anyone every frame. That's what keeps the mosaic fluid
with fifteen live cells.

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
