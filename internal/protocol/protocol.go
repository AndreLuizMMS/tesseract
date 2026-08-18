// Package protocol is the contract between the engine and the screen. One
// message per line, plain JSON, so the screen never needs to know how the
// engine stores things — only what it says to draw.
package protocol

import "encoding/json"

// Message types. The screen sends the first four; the engine sends the last
// two.
const (
	TypeKey       = "key"
	TypeSize      = "size"
	TypeScroll    = "scroll"
	TypeCreate    = "create"
	TypeKill      = "kill"
	TypeRename    = "rename"
	TypeResume    = "resume"
	TypeTab       = "tab"
	TypePrompt    = "prompt"
	TypeComplete  = "complete"
	TypeScreen    = "screen"
	TypeSearch    = "search"
	TypeDocker    = "docker"
	TypeEditor    = "editor"
	TypeGoToLine  = "go-to-line"
	TypeStatus    = "status"
	TypeStop      = "stop"
	TypeState     = "state"
	TypeCompleted = "completed"
	TypeMatches   = "matches"
	TypeServices  = "services"
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

// Complete asks the engine to complete a typed path.
type Complete struct {
	Path     string `json:"path"`
	DirsOnly bool   `json:"dirsOnly"`
}

// Completed is the reply: the completed path and how many candidates
// matched.
type Completed struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
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

// Editor opens the project's directory in the configured editor.
type Editor struct {
	Project string `json:"project"`
}

// GoToLine moves the cell's reading to a history line, which is where the
// search ends up.
type GoToLine struct {
	Cell string `json:"cell"`
	Line int    `json:"line"`
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
	Cells  []Cell `json:"cells"`
}

// State is the whole portrait. The engine sends this whenever anything
// changes.
type State struct {
	Projects []Project  `json:"projects"`
	Types    []CellType `json:"types"`
	Screen   string     `json:"screen,omitempty"`
	Quota    *Quota     `json:"quota,omitempty"`
	Notice   string     `json:"notice"`
}

// Quota is the 5-hour window usage shown in the title bar.
type Quota struct {
	Percent  int    `json:"percent"`
	Rollover string `json:"rollover"` // how long until the window rolls over, already formatted
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
