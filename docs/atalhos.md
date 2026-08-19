# Shortcuts

The whole map comes straight from the code: no key that doesn't exist shows up here.

## Keyboard

**Move — only arrows, no letters**

| Key | Action |
|---|---|
| `↑` `↓` | previous / next cell, crossing projects |
| `←` `→` | previous / next project |
| `space` | jumps to the next cell asking for attention, crossing projects |
| `1`…`9` | goes straight to project N |
| `tab` | switches the cell's tab: claude, cursor, shell, md (`shift-tab` goes back) |

**Keyboard and screen**

| Key | Action |
|---|---|
| `↵` | enters TYPE mode on the focused cell |
| `ctrl-l` | gives the keyboard back to the app |
| `o` | focused cell in full screen (toggles on and off) |
| `v` | switches mosaic ↔ list |

**Create, kill, rename**

| Key | Action |
|---|---|
| `n` | create — a single form, that starts at your home dir and doesn't ask the type |
| `r` | resumes a stopped cell, or restarts a crashed one |
| `D` | kills the focused cell — always confirms |
| `R` | renames the cell **and propagates the name into the agent** |
| `ctrl-r` | adopts, on the cell, the name the agent gave the conversation |

**Act and read**

| Key | Action |
|---|---|
| `p` | sends a prompt to the focused cell without entering it |
| `d` | opens the Docker panel for the focused project |
| `ctrl-e` | opens a picker with every IDE found on the machine, then opens the project's directory in the one you choose |
| mouse wheel | scrolls the cell's history |
| drag with mouse | marks a section of the cell and **copies on release** |
| `/` | searches the focused cell's history |
| `esc` | exits scrolling / closes whatever is open |
| `?` | help |
| `q` | closes the screen — the engine keeps running |

## Commands

```
ts                 opens the screen, starting the engine if needed
ts novo <dir>      adds a project without opening the screen
ts status          engine state and summary of projects and cells
ts stop            shuts down the engine and all cells
ts reset           deletes the saved state and tears everything down, keeping the config
```

## Recovery

`wsl --shutdown` kills every process. When WSL comes back, the service starts by itself and
rebuilds the grid:

| Type | What happens |
|---|---|
| `session` | comes back on the same tab, with each agent's conversation resumed and **stopped**. No prompt is fired |
| `bash` tab | new, clean shell; the previous history stays scrollable above the drop line |
| `logs` | goes back to following the service; if the stack is stopped, it hooks in by itself once it comes up |
| `md` | rereads the file |
| Docker | **doesn't come up by itself**. Bringing up a stack is your call |

The risk that automatic recovery creates — an agent working with nobody in front of the
screen — is cut at the root: **resuming a conversation never fires off work**. There's a
test that fails if a single byte gets written to the agent's keyboard during
reconstitution.

## Notification

The engine notifies, not the screen. Sound and system notification with the cell's and the
project's name, even with the screen closed. On WSL, the toast goes out through Windows
PowerShell; if you have `wsl-notify-send.exe` or `notify-send`, those are used instead. Both
notifications can be turned off, separately.
