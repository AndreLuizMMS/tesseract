// Package protocol is the contract between the engine and the screen. One
// message per line, plain JSON, so the screen never needs to know how the
// engine stores things — only what it says to draw.
package protocol

import "encoding/json"

// Message types. The screen sends the first four; the engine sends the last
// two.
const (
	TypeKey      = "key"
	TypeSize     = "size"
	TypeScroll   = "scroll"
	TypeCreate   = "create"
	TypeKill     = "kill"
	TypeRename   = "rename"
	TypeResume   = "resume"
	TypeTab      = "tab"
	TypePrompt   = "prompt"
	TypeAnswer   = "answer"
	TypeComplete = "complete"
	TypeScreen   = "screen"
	TypeSearch   = "search"
	TypeDocker   = "docker"
	TypeEditor   = "editor"
	TypeListIDEs = "list-ides"
	TypeGoToLine = "go-to-line"
	TypeOpenDoc  = "open-doc"
	TypeStatus   = "status"
	TypeStop     = "stop"
	// TypeAccounts has the engine read the Claude accounts in the config
	// again: one set up by `ts account add` is used without a restart.
	TypeAccounts  = "accounts"
	TypeReport    = "report"
	TypeLink      = "link"
	TypeDeliver   = "deliver"
	TypeState     = "state"
	TypeCompleted = "completed"
	TypeMatches   = "matches"
	TypeServices  = "services"
	TypeIDEs      = "ides"
	TypeSummary   = "summary"
	TypeError     = "error"
)

// Key carries a key into the cell. It carries the key, not its bytes:
// knowing how to translate a key into bytes is the job of the cell's inner
// screen, which knows the terminal's modes down there.
type Key struct {
	Cell  string `json:"cell"`
	Code  rune   `json:"code"`
	Text  string `json:"text,omitempty"`
	Mod   int    `json:"mod,omitempty"`
	Paste string `json:"paste,omitempty"` // text pasted all at once
}

// Size tells the engine the area the screen reserved for the cell, so the
// process down there sees the terminal at the right size.
type Size struct {
	Cell string `json:"cell"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// Scroll moves the cell's history reading. Live true returns the reading to
// the end, ignoring the delta.
type Scroll struct {
	Cell  string `json:"cell"`
	Delta int    `json:"delta"`
	Live  bool   `json:"live"`
}

// Create creates a cell. If the path isn't on screen yet, the project gets
// created along with it.
type Create struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Target string `json:"target"`
	Prompt string `json:"prompt"`
}

// Kill removes the cell. The disk is never touched.
type Kill struct {
	Cell string `json:"cell"`
}

// Rename asks the agent to choose and write the conversation's own name;
// the engine applies that name to the cell when the turn ends.
type Rename struct {
	Cell string `json:"cell"`
}

// Tab switches the active tab of a cell that has several. A positive step
// moves right.
type Tab struct {
	Cell string `json:"cell"`
	Step int    `json:"step"`
}

// Resume brings a stopped or dead cell back up.
type Resume struct {
	Cell string `json:"cell"`
}

// Prompt sends work to the cell without entering it.
type Prompt struct {
	Cell string `json:"cell"`
	Text string `json:"text"`
}

// Answer settles, from the mosaic, the tool approval the focused cell is
// blocked on: approve it or decline it, without entering the cell. The engine
// refuses when the cell isn't waiting on exactly that.
type Answer struct {
	Cell    string `json:"cell"`
	Approve bool   `json:"approve"`
}

// Complete asks the engine to complete a typed path.
type Complete struct {
	Path     string `json:"path"`
	DirsOnly bool   `json:"dirsOnly"`
	// List asks only for the candidates, leaving what was typed alone — it's
	// what the creation form's live folder list uses on every keystroke.
	List bool `json:"list,omitempty"`
}

// Completed is the reply: the completed path, how many candidates matched
// and the candidates themselves, so the screen can show them.
type Completed struct {
	Path    string   `json:"path"`
	Count   int      `json:"count"`
	Options []string `json:"options,omitempty"`
}

// Screen remembers which screen the user picks across runs.
type Screen struct {
	Screen string `json:"screen"`
}

// Search searches for a term in the focused cell's history.
type Search struct {
	Cell string `json:"cell"`
	Term string `json:"term"`
}

// Match is one history line that matched the search.
type Match struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}

// Matches is the search's reply.
type Matches struct {
	Cell  string  `json:"cell"`
	Term  string  `json:"term"`
	Lines []Match `json:"lines"`
}

// Docker acts on the stack or on one of the project's services. Nothing
// here is destructive: no tearing down with a volume, no deleting anything.
type Docker struct {
	Project string `json:"project"`
	Action  string `json:"action"`            // up, down, restart, rebuild, log
	Service string `json:"service,omitempty"` // empty when the action is for the whole stack
}

// Service is one row of the Docker panel.
type Service struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Port   string `json:"port,omitempty"`
	Health string `json:"health,omitempty"`
	Uptime string `json:"uptime,omitempty"`
}

// Services is the engine's reply to a request to list the project's stack.
// Action says which request this reply came from, so the screen knows when
// the work it showed as in progress has finished.
type Services struct {
	Project string    `json:"project"`
	File    string    `json:"file"`
	Action  string    `json:"action,omitempty"`
	Service string    `json:"service,omitempty"`
	List    []Service `json:"list"`
	Error   string    `json:"error,omitempty"`
}

// Editor opens the project's directory in the picked IDE.
type Editor struct {
	Project  string `json:"project"`
	ID       string `json:"id"`
	Location string `json:"location"`
	// Cell, when set, asks for the last file that cell's agent edited, at
	// the line, instead of the bare directory.
	Cell string `json:"cell,omitempty"`
}

// ListIDEs asks the engine which IDEs are reachable from here to open the
// project in.
type ListIDEs struct {
	Project string `json:"project"`
}

// IDE is one editor tesseract found on the machine — either installed
// inside WSL or reachable through the Windows side.
type IDE struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Location string `json:"location"` // "wsl" or "windows"
}

// IDEs is the engine's answer to ListIDEs.
type IDEs struct {
	Project string `json:"project"`
	List    []IDE  `json:"list"`
	Error   string `json:"error,omitempty"`
}

// GoToLine moves the cell's reading to a history line, which is where the
// search ends up.
type GoToLine struct {
	Cell string `json:"cell"`
	Line int    `json:"line"`
}

// OpenDoc opens, in the cell's markdown reader, the file its agent last
// wrote.
type OpenDoc struct {
	Cell string `json:"cell"`
}

// Report is what the mod running inside a Claude Code cell tells the engine
// about its own session. It replaces guessing from the cell's screen: the
// agent says what turn state it's in, which conversation it's holding and
// how much of the usage window is gone.
type Report struct {
	Cell string `json:"cell"`
	Tab  string `json:"tab,omitempty"`
	// Kind picks which of the fields below matter: "state", "session",
	// "quota", "limit", "title", "delivered", "doc" or "edit". A "limit" is
	// the account's usage running out mid-turn, with ResetsAt saying when it
	// comes back, when the mod knows.
	Kind string `json:"kind"`
	// Delivery is the ID of the Deliver the mod is answering, and Refused
	// says its session had nowhere to put it.
	Delivery string `json:"delivery,omitempty"`
	Refused  bool   `json:"refused,omitempty"`
	// State is working, idle, answered, approve, interrupted or ended.
	State string `json:"state,omitempty"`
	// Ask says what an approve is waiting for: "tool" (a permission dialog,
	// which enter approves and esc declines), "question" or "plan".
	Ask string `json:"ask,omitempty"`
	// Summary is the answer in one short line, when the mod was asked to
	// write one. It takes the detail's place in the notice and shows on the
	// cell's header.
	Summary string `json:"summary,omitempty"`
	// Detail is what the notice says beyond the state: the tool and command
	// waiting for approval, or the opening of the answer.
	Detail string `json:"detail,omitempty"`
	// Session is the conversation's real identity, which changes under the
	// cell on /clear, /resume and /branch.
	Session string `json:"session,omitempty"`
	// Percent and ResetsAt are the 5-hour window, the latter in RFC 3339.
	Percent  float64 `json:"percent,omitempty"`
	ResetsAt string  `json:"resetsAt,omitempty"`
	// Weekly is the 7-day window, riding on a "quota" report. Context is how
	// full the conversation's own window is, in a report of kind "context":
	// that one belongs to the cell, not to the account.
	Weekly  float64 `json:"weekly,omitempty"`
	Context float64 `json:"context,omitempty"`
	// At is the mod's clock in milliseconds when the event happened. Each
	// report travels in its own process, and this is what keeps a late one
	// from overwriting a newer state.
	At int64 `json:"at,omitempty"`
	// File is the absolute path the agent just wrote: a markdown to read
	// ("doc"), or any file, with the Line the change starts on ("edit").
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
}

// Link turns the connection into the engine's way of reaching the mod of
// one cell: from here on it receives Deliver messages and no snapshot.
type Link struct {
	Cell string `json:"cell"`
	Tab  string `json:"tab,omitempty"`
}

// Deliver is work the engine hands to a cell's mod, to be submitted as the
// user's own words. A text that opens with a slash is a command.
// The mod answers with a "delivered" report carrying the ID, and ignores a
// delivery whose Expires (milliseconds) has passed: the engine has given up
// on it and typed the text into the cell instead.
type Deliver struct {
	ID      string `json:"id,omitempty"`
	Text    string `json:"text"`
	Expires int64  `json:"expires,omitempty"`
	// Event marks a line that carries no work: "attached" is the engine
	// saying the link is up, which is when the mod's word about its state
	// starts to count — so the mod says it again.
	Event string `json:"event,omitempty"`
}

// CellType is a type's spec, as the form needs to know it. Comes from the
// engine so the screen never needs to know which types exist.
type CellType struct {
	Type          string `json:"type"`
	TargetLabel   string `json:"targetLabel,omitempty"`
	CompletesPath bool   `json:"completesPath,omitempty"`
	AcceptsPrompt bool   `json:"acceptsPrompt,omitempty"`
	Chat          bool   `json:"chat,omitempty"`
}

// Summary is the reply to a status request, in text ready for the terminal.
type Summary struct {
	Text string `json:"text"`
}

// Error is what the engine returns when a request couldn't be fulfilled.
type Error struct {
	Message string `json:"message"`
}

// Cell is a cell's portrait, ready to draw.
type Cell struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	State   string   `json:"state"`
	Lines   []string `json:"lines"`
	CursorX int      `json:"cursorX"`
	CursorY int      `json:"cursorY"`
	Scroll  int      `json:"scroll"`
	Live    bool     `json:"live"`
	// Tabs and Tab exist on cells that hold more than one agent inside.
	// The screen draws the tabs in place of the type.
	Tabs []string `json:"tabs,omitempty"`
	Tab  string   `json:"tab,omitempty"`
	// Asleep are the tabs whose process was released for being silent out of
	// view. Nothing was lost — switching to one brings the agent back where
	// it left off — but the screen says so instead of letting a process
	// vanish without a word.
	Asleep []string `json:"asleep,omitempty"`
	// Doc is a markdown the cell's agent wrote that nobody opened yet, and
	// Edited the last file it changed, as "path:line". Both are relative to
	// the project and both come from the cell's mods.
	Doc    string `json:"doc,omitempty"`
	Edited string `json:"edited,omitempty"`
	// Detail is what the cell's agent said about the state it's in: what an
	// approve is waiting for, or the answer in one line. Ask is the kind of
	// approve ("tool", "question", "plan"); only "tool" is settled from the
	// mosaic. Both are empty for an agent with no mod speaking for it.
	Detail string `json:"detail,omitempty"`
	Ask    string `json:"ask,omitempty"`
	// Context is how full the conversation's window is, in percent. Zero
	// when nobody measured it.
	Context int `json:"context,omitempty"`
}

// Project is one column of the mosaic.
type Project struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Name       string `json:"name"`
	Color      int    `json:"color"`
	HasCompose bool   `json:"hasCompose"`
	// Docker is the short summary of the stack for the strip: empty with no
	// compose, "down", or "4/5" when there are services up.
	Docker string `json:"docker,omitempty"`
	// DockerDown says a service of the stack fell on its own — exited on an
	// error, or dead. It's the one thing about the stack the mosaic shows
	// without being asked.
	DockerDown bool   `json:"dockerDown,omitempty"`
	Cells      []Cell `json:"cells"`
}

// State is the whole portrait. The engine sends this whenever anything
// changes.
type State struct {
	Projects []Project `json:"projects"`
	// Recents are the directories most recently opened, newest first — what
	// the creation form offers before anything is typed.
	Recents []string   `json:"recents,omitempty"`
	Types   []CellType `json:"types"`
	Screen  string     `json:"screen,omitempty"`
	Quota   *Quota     `json:"quota,omitempty"`
	Notice  string     `json:"notice"`
}

// Quota is the 5-hour window usage shown in the title bar.
type Quota struct {
	Percent  int    `json:"percent"`
	Rollover string `json:"rollover"` // how long until the window rolls over, already formatted
	// Weekly is the 7-day window. Zero when no mod reported it.
	Weekly int `json:"weekly,omitempty"`
}

// Message is the envelope that travels over the socket.
type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Pack wraps a value in its type's envelope.
func Pack(typ string, data any) (Message, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Message{}, err
	}
	return Message{Type: typ, Data: raw}, nil
}

// Unpack pulls the value out of the envelope.
func Unpack[T any](m Message) (T, error) {
	var v T
	err := json.Unmarshal(m.Data, &v)
	return v, err
}
