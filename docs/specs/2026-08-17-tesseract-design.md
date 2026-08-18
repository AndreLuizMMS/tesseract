# Tesseract — design specification

**Date:** 2026-08-17
**Status:** approved conceptually, ready to become an implementation plan
**Replaces:** personal fork of `claude-squad` (`~/claude-squad-andre`) — only the concept
survives from the fork; no code is reused.

---

## 1. Why it exists

The current fork solved the right problem — following several agents at once without
entering each one — but it grew by accumulation. Every new feature fought over the same
keyboard, created its own screen path and its own rule. Today the list and the mosaic
disagree with each other, the same key does different things depending on the panel, and
Docker is treated as if it belonged to a session when it actually belongs to the project.

Tesseract is born of the same need, with the explicit decision to **standardize before
growing**: a single unit, a single rule, a single place where decisions live.

Long-term goal stated by the user: **replace the IDE**. He already works mostly inside
agents and only opens the IDE to read markdown, review a PR diff and look at the git tree.
V1 tackles the first of those three.

### 1.1 Context constraints

- Runs on **WSL** on top of Windows. Processes die without warning (`wsl --shutdown`,
  machine sleep, Windows restart). Losing work because of that is unacceptable.
- Every one of the user's projects has a **Docker Compose** running.
- The user **doesn't read code**. The deliverable is a working, finished application;
  architecture, folder structure and stack are the implementer's call.
- The application needs to accept new features without degenerating. There won't be a
  second Frankenstein.

---

## 2. Name

**Tesseract.** Command `tess`.

A tesseract is the hypercube: a grid extended by one more dimension — which is exactly the
product's screen, cells inside projects inside a grid. It shares a root with *tessera*, the
individual piece of a Roman mosaic.

The command is `tess` and not `tesseract` because `tesseract` is the name of Google's OCR
engine. Neither is taken on this machine today, but the collision is avoidable.

---

## 3. Conceptual model

There are exactly **three things**. Nothing else.

### 3.1 Project

A versioned directory — an application the user works on.

Born together with its first cell — creating a project and creating a cell are the same
gesture, because a project without a cell makes no sense. **Disappears from the screen when
its last cell dies.**

**The disk is never touched**: a project leaving the screen doesn't delete, move or change
anything.

Keeps: path, column color, screen order, detected compose file (if any), and the cells that
live in it.

### 3.2 Cell

The unit of work. Every cell has a type, a name, a state, a position in the column and a
history. It's born, lives, asks for attention, dies.

**The same rule applies to all of them.** The same key creates, kills, renames, focuses and
navigates any cell, whether it's an agent talking or a markdown file sitting still.

V1 types:

| Type | What it is | Family |
|---|---|---|
| `claude` | Claude Code in the project's directory | has a process, interactive |
| `cursor` | Cursor CLI in the project's directory | has a process, interactive |
| `bash` | shell in the project's directory | has a process, interactive |
| `logs` | live log of a compose service | has a process, read-only |
| `md` | rendered markdown file, reloads when the disk changes | no process, read-only |

Two families on the inside, **just one on the outside**. The distinction between "has a
process" and "doesn't have a process" is an implementation detail and never shows up to the
user as a different rule.

There's no limit on cells per project. The fork had a ceiling of ten sessions; the side
strip and the column's scroll handle overflow without inventing an arbitrary number.

### 3.3 Docker panel

Belongs to the project, isn't a cell. Exists when the project has a compose file at the
root. It isn't created and it isn't killed.

It was the fork's clearest conceptual collision: there, Docker belonged to the session, so
two agents in the same project produced two panels for the same compose file, with no
answer to "which one do I use to bring the stack down".

---

## 4. Cell state

Turn state only makes sense for whoever talks. A shell doesn't "answer".

| State | Who has it | Meaning |
|---|---|---|
| `▸ working` | all | live process producing |
| `⬤ answered` | `claude` `cursor` | gave back its turn, has a response waiting to be read |
| `⏵ approve` | `claude` `cursor` | stuck on a question and **won't move** without an answer |
| `✖ crashed` | with a process | the process died on its own |
| `○ stopped` | all | no process, cell preserved |
| `⚠ orphan` | all | the project's directory disappeared from disk |

**Answered ≠ approve.** That's the distinction that makes the alarm worth something, and
it's inherited from the fork because it works. An agent stuck on a question blocks the
work; an agent that finished its turn just has something to read.

**No false alarms.** A blinking spinner and a moving cursor don't count as activity. The
engine requires consecutive reads of work before considering the cell "armed", and
consecutive reads of silence before declaring the turn over — the same criterion that
already proved itself in the fork.

**State read from the process, not from text on screen.** Detecting that the agent crashed
by looking for words in the output is fragile; the engine watches the process.

### 4.1 Cell name

Every cell is born with an automatic name (`claude 1`, `bash tests`). The user renames it
whenever they want.

A `claude` cell can **adopt the name Claude itself gave the conversation**, without typing
anything. A name chosen by hand inside the agent takes priority over the automatic title
the agent generates. If the conversation doesn't have a name yet, the action warns instead
of renaming to empty.

Name is a label. Changing the name doesn't touch the process, the directory or the
history.

### 4.2 Identity

Every cell has a stable identity that **isn't the process**. The engine keeps the intent —
this project, this cell, of this type, with this name, in this position, tied to this
conversation.

The process dies; the identity stays. That's what allows everything to be reconstituted
after a WSL crash without losing work.

---

## 5. Screens

Three screens and a form.

### 5.1 Mosaic — main screen

All projects at once, each project its own column.

```
 TESSERACT            ⬤ 3   ⏵ 1          NAVIGATE      ⏳ 59% 2:47
┌───┬──────────────── CORTZ-WEB ─────────────────────┬───┐
│ D │ ┌ claude · fix nav ─────────────── ⬤ ANSWERED  ┐│ A │
│ O │ │ Adjusted the Header to collapse below 768px. ││ P │
│ X │ │ Build passed. Should I cover the mobile       ││ P │
│ A │ │ menu too?                                     ││ R │
│ R │ └──────────────────────────────────────────────┘│ O │
│   │ ┏ bash · tests ────────────────────── ▸ RUNNING┓│ V │
│ 5 │ ┃ $ pnpm test                                  ┃│ E │
│⬤2 │ ┃   ✓ 12 passed   ✗ 0 failed                   ┃│   │
│⏵1 │ ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛│ 5 │
│ ● │ ┌ md · spec-m7.md ─────────────────────── ○ ───┐│⬤2 │
│4/5│ │ # Module 7 — Clinical records                 ││⏵1 │
│   │ │ Log an appointment with PHI…                  ││ ● │
└───┴──────────────────────────────────────────────────┴───┘
 ↑↓ cell   ←→ project   ↵ type   n create  d docker   ? help
```

**Width rule.** The focused project's column takes up a real reading width, with the
cells' live content. The other columns shrink to a narrow strip.

**The strip never disappears.** It shows, top to bottom: the project's name vertically, the
number of cells, how many are asking for attention, and Docker's state. **The text
disappears, never the signal.** That preserves the overview that motivated choosing a
global mosaic, without the cost of unreadable cells with three projects open.

**Navigating between projects** means moving to the neighboring column: it fattens, the
current one shrinks. That gesture is what "entering and leaving a project" means — there's
no separate project screen.

**One border, one meaning.** The fork had two thick borders with different meanings (where
the arrows are and where typed input goes). With the two explicit modes, focus and
keyboard are always the same cell, and one border is enough.

When a project's cells don't fit the height, the column scrolls and the strip signals there
is more.

### 5.2 The mode is impossible to get wrong

In `TYPE` mode the app goes mute and **shows that it's mute**.

```
 tesseract            ⬤ 3   ⏵ 1          ▓ TYPE ▓        ⏳ 59% 2:47
┌───┬───────────────── cortz-web ────────────────────┬───┐
│ d │ ┌ claude · fix nav ──────────────── ⬤ answered ┐│ a │
│ o │ │ Adjusted the Header to collapse below 768px. ││ p │
│ x │ └──────────────────────────────────────────────┘│ p │
│ a │ ┏━ bash · tests ━━━━━━━━━━━━━━━━━━━━━ ▸ RUNNING┓│ r │
│ r │ ┃ $ pnpm test                                  ┃│ o │
│   │ ┃   ✓ 12 passed   ✗ 0 failed                   ┃│ v │
│ 5 │ ┃ $ █                                          ┃│ e │
│   │ ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛│   │
└───┴──────────────────────────────────────────────────┴───┘
 ctrl-l returns the keyboard
```

Three redundant signals at once: bar and strips dimmed, `▓ TYPE ▓` badge in the title bar,
and the footer reduced to one line. The focused cell is the only thing lit.

The redundancy is deliberate: getting the mode wrong means typing a command in the agent's
face.

### 5.3 List — the index

Same content, no video. Serves to scan many projects, find things fast, and work on a
short terminal.

```
 TESSERACT            ⬤ 3   ⏵ 1          NAVIGATE      ⏳ 59% 2:47
┌────────────────────────────┬─────────────────────────────┐
│ DOXAR-API      ~/dev/doxar │ claude · refactor auth      │
│  ⬤ claude  refactor auth   │                             │
│  ⏵ claude  migrate prisma  │ Moved the token check to    │
│  ▸ cursor  review PR       │ the guard. Still need to    │
│  ○ bash    tests           │ decide if the refresh goes  │
│  ▸ logs    worker          │ in the same flow or becomes │
│  ● docker  4/5 up          │ its own endpoint.           │
│                            │                             │
│ CORTZ-WEB      ~/dev/cortz │ Which one do you prefer?    │
│▸ ⬤ claude  fix nav         │                             │
│  ▸ bash    tests           │                             │
│  ○ md      spec-m7.md      │                             │
│  ○ docker  stopped         │                             │
└────────────────────────────┴─────────────────────────────┘
 ↑↓ navigate   ↵ type   n create  v mosaic   d docker   ? help
```

Index on the left, live preview of the selected cell on the right. `↵` enters `TYPE` mode
in the preview, without leaving the list.

The chosen screen (list or mosaic) is remembered across runs.

### 5.4 Docker panel

One key opens the focused project's Docker view, over the screen. It closes and returns
exactly where it was.

```
┌──────────── DOCKER · doxar-api ──────── docker-compose.yml ─┐
│                                                             │
│    SERVICE      STATE          PORT     HEALTH    UPTIME   │
│  ▸ api          ● up           :3000    healthy    2h14m    │
│    postgres     ● up           :5432    healthy    2h14m    │
│    redis        ● up           :6379    —          2h14m    │
│    worker       ○ exited (1)   —        —          —        │
│    minio        ● up           :9000    —          2h14m    │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  ↑↓ pick a service                                          │
│                                                             │
│  ON SERVICE   u up   s stop   r restart   b rebuild         │
│               l opens the log as a cell                     │
│  ON STACK     U up all   S stop all   R restart all         │
│                                                             │
│  esc closes                                                 │
└─────────────────────────────────────────────────────────────┘
```

`↑↓` is still navigation, like everywhere else in the app. Actions are letters, and the
**uppercase is always the whole-stack version** of the matching lowercase.

**While the panel is open, the keyboard is its own** — same logic as the two modes: never
two owners of the keyboard at once. `esc` gives it back.

**A service's log becomes a cell.** Asking for `worker`'s log creates a `logs`-type cell in
that project's mosaic — with the same navigation, the same focus, the same kill key and the
same history as any other cell. That way the database's log can stay pinned beside the
agent debugging it.

**No destructive action exists here.** There's no `down -v`, no deleting a volume. If it's
ever needed, it comes with a written confirmation.

**Compose detection:** only at the project directory's root, no recursive search. A project
without compose simply has no Docker panel, and the strip shows no indicator.

### 5.5 Creation form

**A single form, for everything.** There's no "create project" separate from "create cell":
the form starts by asking for the project and ends by creating the cell.

```
┌──────────────────────── NEW ─────────────────────────┐
│                                                       │
│  PROJECT ~/dev/cortz-web▏                             │
│          tab completes · a new path creates a project │
│                                                       │
│  TYPE    ▸claude    cursor    bash    logs    md      │
│                                                       │
│  NAME    fix the mobile menu_                         │
│          (empty → automatic name)                     │
│                                                       │
│  MD      docs/spec-m7.md▏                             │
│          tab completes · 3 files match                │
│                                                       │
│  PROMPT  (optional — starts already working)          │
│          ▏                                            │
│                                                       │
│  ↵ create        esc cancel                           │
└─────────────────────────────────────────────────────┘
```

- **PROJECT** comes pre-filled with the focused project. Typing a path not yet on screen
  creates the project right there. Autocomplete with `tab`, showing how many folders
  match. The path is validated live — it needs to exist and be writable; if it isn't, the
  field stays open with what was typed.
- The fourth field depends on the type: `md` → file with autocomplete; `logs` → compose
  service picker; `claude` `cursor` `bash` → disappears.
- **PROMPT** only shows up for `claude` and `cursor`, and covers "create already working",
  which was a separate key in the fork.
- The agent profile used by `claude` and `cursor` comes from the configuration, with the
  default listed first.

---

## 6. Modes and keyboard

### 6.1 Principle

One key, one meaning, on any screen and in any project. If `D` kills a cell in the mosaic,
it kills a cell in the list. No exception per panel — that's exactly what broke in the
fork.

### 6.2 The two modes

By default the user is in **NAVIGATE**: every key belongs to the app.

One key enters **TYPE**: every key belongs to the cell, **with no exception whatsoever**.
Not `q`, not `D`, not `tab`, not the arrows. One key gives the keyboard back to the app.

There's never two owners of the keyboard at the same time, so a collision is structurally
impossible.

### 6.3 NAVIGATE MODE

**Move — only arrows, no letters**

| Key | Action |
|---|---|
| `↑` `↓` | previous / next cell in the column |
| `←` `→` | previous / next project (the column fattens) |
| `space` | jumps to the next cell asking for attention — crosses projects |
| `1`…`9` | goes straight to project N |

Navigation is exclusively directional. No letter moves around the grid, so the movement
vocabulary stays single and doesn't compete with action.

**Keyboard and screen**

| Key | Action |
|---|---|
| `↵` | enters TYPE mode on the focused cell |
| `ctrl-l` | leaves TYPE mode, gives the keyboard back to the app |
| `o` | focused cell full screen (on/off); `esc` also exits |
| `v` | switches mosaic ↔ list |

**Create and kill**

| Key | Action |
|---|---|
| `n` | create — asks for the project, then the cell (single form, §5.5) |
| `r` | resumes a stopped cell, or restarts a crashed one |
| `D` | kills the focused cell — confirms. If it's the project's last one, the confirmation warns that the project will leave the screen |

There's no key to close a project: the project leaves the screen when its last cell dies.

**Rename**

| Key | Action |
|---|---|
| `R` | renames the cell **and propagates the name into the agent** — `/rename` in Claude Code, the equivalent command in Cursor CLI. In `bash`, `logs` and `md` it just changes the label |
| `ctrl-r` | adopts, on the cell, the name the agent gave the conversation |

Both directions coexist without getting in each other's way: `R` pushes the user-chosen
name into the agent; `ctrl-r` pulls into the cell the name the agent chose on its own.

**Act**

| Key | Action |
|---|---|
| `p` | sends a prompt to the focused cell without entering it — works for `claude` and for `cursor`, depending on the cell's type |
| `d` | opens the focused project's Docker panel |
| `ctrl-e` | opens the project's directory in the configured IDE (`cursor /path`) |

**Read**

| Gesture | Action |
|---|---|
| mouse wheel | scrolls the history of the cell under the cursor |
| `/` | searches the focused cell's history |
| `esc` | exits scrolling / closes whatever is open |

**System**

| Key | Action |
|---|---|
| `?` | help |
| `q` | closes the screen. The engine keeps running — nothing dies |

### 6.4 TYPE MODE

| Key | Action |
|---|---|
| `ctrl-l` | gives the keyboard back to the app |
| everything else | goes to the cell, no exception |

The mouse wheel keeps scrolling the history, because it isn't a key.

### 6.5 Selecting and copying text

The app holds the mouse to receive the wheel, and that turns off the terminal's native
selection. The standard escape is **`shift` + drag**, which Windows Terminal respects.

In the mosaic, though, `shift` + drag grabs the neighbors: the same screen line crosses
several columns. To copy a block of text, `o` puts the cell full screen and selection then
only grabs it.

That's why **there's no key for "give the mouse back to the terminal"** — the fork needed
one; here full screen solves it without spending a key.

### 6.6 Free keys, on purpose

Lowercase: `a` `b` `c` `e` `f` `g` `h` `i` `j` `k` `l` `m` `s` `t` `u` `w` `x` `y` `z`
Uppercase: all but `D` and `R`
Plus `tab` and `ctrl-` on almost every letter.

Nineteen free lowercase keys. A new feature doesn't need to steal anyone's key.

### 6.7 Review of what exists in the fork today

**Stays the same**

Turn states and the answered ≠ approve distinction · heuristic against false alarms ·
sound and notification, both toggleable with a custom command · `space` jumps to whoever
called · `p` sends a prompt without entering · `ctrl-r` adopts the agent's name · `ctrl-e`
opens in the editor · 5h-window usage badge · configurable agent profiles · directory
autocomplete with `tab` · grouping by project · the chosen screen is remembered.

**Changes**

| Today | In Tesseract | Why |
|---|---|---|
| `tab` switches panel inside the session | dies | the panel became a cell; nothing left to switch |
| `n` creates a session, `N` creates with a prompt | `n` creates everything: project and cell, in a single form | a project without a cell makes no sense; the prompt became a field |
| `o` = synonym for `↵` | `o` = full screen | `↵` already enters; and full screen becomes the way to copy text without grabbing the neighbors |
| `↑↓` and `jk` navigate | only `↑↓` | no letter moves around the grid; movement is directional, period |
| `R` renames only the label | `R` renames **and propagates into the agent** | the cell's name and the conversation's name stop diverging |
| `shift-↑↓` scrolls the history | mouse wheel | scrolling is a mouse gesture; frees up keys |
| `D` in the mosaic kills without confirming | always confirms | same key, same rule, on every screen |
| diff `+/-` per session | per project, in the column header | it's a single repository |
| Docker is a session tab | project panel + log cell | the compose belongs to the project |
| Notification comes from the screen | comes from the engine | it warns even with the screen closed |

**Dies**

| What | Why |
|---|---|
| `max_sessions` (ceiling of 10) | artificial limit; the strip and the scroll handle overflow |
| Auto-yes and the daemon that approves on its own | approving without reading is exactly what the `⏵ approve` state exists to prevent. Can come back on request |
| `ctrl-space` (give the mouse back to the terminal) | `o` + `shift` + drag covers the case without spending a key (§6.5) |
| `J` `K` `H` `L` (reorder) | order is creation order. Reordering is out of V1 and can come back without spending a new key, if the need shows up |
| `X` (close project) | the project leaves the screen when its last cell dies |
| `restart.sh` / `clean.sh` / `clean_hard.sh` | become `tess reset` |

---

## 7. The engine

### 7.1 What it is

A user service running on WSL. It comes up together with WSL, survives closing the screen,
and owns everything: the processes, each cell's internal screen, the history and the
desired state.

The screen is a **client**. It connects to the engine, draws what it's told, returns keys.
No rule lives in the screen.

The engine keeps each cell's internal screen in memory — it **already knows** what's
written in each one, without needing to ask anyone every frame. That's what keeps the
mosaic fluid with fifteen live cells, and it's the central difference from the fork, which
needed to query tmux every round.

More than one screen can be connected at the same time, in different terminals, watching
the same state.

### 7.2 What lives on disk

| What | Behavior |
|---|---|
| Desired state | projects, cells, type, name, position, linked conversation. Atomic write. If it corrupts, loads what it can, preserves the original file and warns on screen |
| Per-cell history | its own file, with a size ceiling and discard from the start. It's what backs scrolling and search after a crash |
| Configuration | agent profiles, editor, sound, notification, history ceiling |

### 7.3 Recovery

Scenario: `wsl --shutdown`. Every process dies.

The user reopens WSL. The service comes up by itself and rebuilds the grid in the same
positions:

| Type | What happens |
|---|---|
| `claude` `cursor` | resumes the conversation where it left off and wakes up **stopped**. No prompt is fired |
| `bash` | new, clean shell; the previous history stays scrollable above the drop line |
| `logs` | goes back to following the service; if the stack is stopped, it stays `○ stopped` and hooks in by itself once the service comes up |
| `md` | rereads the file |
| Docker | **doesn't come up by itself**. Bringing up a stack is the user's call |

The risk that automatic recovery creates — an agent working with nobody in front of the
screen — is cut at the root: **resuming a conversation never fires off work**. The agent
wakes up waiting.

### 7.4 A cell that crashes while the app is running

Turns into `✖ crashed`, sounds the alert, and waits. `r` brings it back up.

**Nothing restarts on its own.** Automatic restart hides the root cause and produces a
silent crash loop.

### 7.5 Notification

The engine is what notifies, not the screen. Sound and system notification with the cell's
and the project's name, detecting the available notification command
(`wsl-notify-send.exe` on WSL), with the option to point to a different command. Both
toggleable.

Since the engine is independent of the screen, the user gets notified even with the screen
closed — which the fork didn't do.

### 7.6 Usage badge

The title bar shows the 5-hour window's usage and the time until it resets. Claude Code
only hands that number to the statusline, so the engine reads it from a file the user's
statusline writes. The badge disappears if the data is stale and changes color above 80%.
Without the file, everything still works — the badge just doesn't show up.

---

## 8. How this doesn't turn into another Frankenstein

Three hard rules. They're what answer the requirement to accept new features.

**1. A cell type is a sealed piece.** A type declares four things: how it's born, how it
draws itself, what it does with a key, and what states it has. Nothing outside it changes.
Adding a git cell, a PR cell, a database cell or anything else means writing that piece —
the mosaic, the list, the shortcuts and the engine aren't touched.

**2. A new feature doesn't get a new key.** The set of keys is closed and lives in one
place. A feature comes in as a cell type, as a Docker panel action, or as a written command.
If it really needs a key, someone loses theirs — a conscious decision, not accumulation. It
was silent accumulation that created the Frankenstein.

**3. The screen doesn't decide anything.** It draws what the engine tells it. A new rule
enters the engine once and shows up on both screens for free. It's structurally impossible
for the list and the mosaic to disagree.

---

## 9. Command line

| Command | What it does |
|---|---|
| `tess` | opens the screen, connecting to the engine (starts the engine if it isn't running) |
| `tess novo <dir>` | adds a project without opening the screen |
| `tess status` | engine state and summary of projects and cells |
| `tess stop` | shuts down the engine and every cell |
| `tess reset` | deletes the saved state and tears everything down, keeping the configuration |

---

## 10. Scope

### 10.1 In V1

Engine as a service, with reconstitution · projects · `claude` `cursor` `bash` `logs` `md`
cells · mosaic with columns and status strip · list with preview · the two modes · Docker
panel with actions on the stack and the service · recorded history with scroll and search ·
notification from the engine · usage badge · the five command-line commands.

### 10.2 Consciously left out

| What | When it comes back |
|---|---|
| Git cell (local tree + diff) | when V1 is in daily use and the need shows up |
| GitHub PR cell | same — it's the most expensive: depends on network, auth and API error handling |
| Auto-yes | if the user confirms they'd actually use it |
| Written command (palette) | when the first feature doesn't fit in the sixteen free keys |
| Visual configuration and themes | until requested |
| Running outside WSL | until needed |

---

## 11. Recorded decisions

| # | Decision | Discarded alternative and why |
|---|---|---|
| 1 | Everything is a typed cell; a single rule to create, kill, rename and focus | Separating "agent" from "infra" creates two interaction rules — the seed of the collision that exists in the fork |
| 2 | A project is a versioned directory and owns Docker | Docker per session duplicates the panel and leaves "who brings the stack down" unanswered |
| 3 | Global mosaic with one project per column; the focused one fattens, the rest becomes a strip | Paging hides exactly the project that called; scrolling sideways has the same flaw with no clear boundary |
| 4 | Engine as a service that comes up with WSL and reconstitutes itself | Reconstitution only when the screen opens leaves the user unwarned while it isn't open |
| 5 | Docker panel lists services and acts; log becomes a cell | A log inside the panel is always ephemeral and creates its own internal navigation |
| 6 | Two explicit modes, nothing reserved in TYPE mode | Reserved keys make every new feature steal a key from the agent |
| 7 | V1 closes with `claude` `cursor` `bash` `logs` `md` | Git and PR are expensive and the typed-cell model makes it cheap to add them later |
| 8 | Engine with its own internal screen per cell | Using tmux as the substrate forces querying an external process every frame and leaves `md` and Docker outside the model |
| 9 | Navigation only by arrow; no letter moves around the grid | `jk`/`hl` make movement and action compete for the same vocabulary, and that's where the feeling of an unpredictable key comes from |
| 10 | Creating a project and creating a cell are a single gesture; the project disappears with its last cell | Having a separate key to open and close a project creates an empty state (a project without a cell) that's good for nothing |
| 11 | `R` propagates the name into the agent | Renaming only the label lets the cell's name and the conversation's name diverge, which is the problem the fork's `ctrl-r` tried to patch from one side only |
| 12 | Scrolling is the mouse wheel; copying is full screen + `shift` | Keeping `shift-↑↓` and `ctrl-space` spent three keys on what two mouse gestures solve |
