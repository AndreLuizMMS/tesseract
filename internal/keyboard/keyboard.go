// Package keyboard is the only place where a key turns into an action. No
// file outside of here ties a key to a meaning — that's the rule that keeps
// the same key from doing different things depending on the screen.
package keyboard

import "strconv"

// Mode is who owns the keyboard right now. There's never two owners at once.
type Mode int

const (
	// Browse: every key belongs to the application.
	Browse Mode = iota
	// Type: every key belongs to the cell, with no exception besides
	// ctrl-l.
	Type
	// Form: the keyboard belongs to the focused field; only command keys
	// get out.
	Form
	// DockerPanel: while the panel is open, the keyboard is its.
	DockerPanel
)

func (m Mode) String() string {
	switch m {
	case Type:
		return "TYPE"
	case Form:
		return "FORM"
	case DockerPanel:
		return "DOCKER"
	}
	return "BROWSE"
}

// Action is what a key means.
type Action string

const (
	None Action = ""

	// Movement — arrows only, no letters.
	CellPrevious Action = "cell-previous"
	CellNext     Action = "cell-next"
	CellUp       Action = "cell-up"
	CellDown     Action = "cell-down"
	SkipToAlert  Action = "skip-to-alert"
	GoToProject  Action = "go-to-project"
	NextTab      Action = "next-tab"
	PreviousTab  Action = "previous-tab"

	// History scrolling — jumps bigger than the mouse wheel.
	ScrollPageUp   Action = "scroll-page-up"
	ScrollPageDown Action = "scroll-page-down"
	ScrollTop      Action = "scroll-top"
	ScrollBottom   Action = "scroll-bottom"

	// Keyboard and screen.
	EnterType    Action = "enter-type"
	ExitType     Action = "exit-type"
	Fullscreen   Action = "fullscreen"
	SwitchScreen Action = "switch-screen"

	// Create and kill.
	Create Action = "create"
	Resume Action = "resume"
	Kill   Action = "kill"

	// Rename.
	Rename Action = "rename"

	// Act.
	Prompt     Action = "prompt"
	OpenDocker Action = "open-docker"
	OpenEditor Action = "open-editor"
	SearchTerm Action = "search-term"
	Back       Action = "back"
	Help       Action = "help"
	Close      Action = "close"

	// Form.
	Confirm       Action = "confirm"
	Cancel        Action = "cancel"
	Complete      Action = "complete"
	PreviousField Action = "previous-field"
	NextField     Action = "next-field"
	PreviousOpt   Action = "previous-option"
	NextOpt       Action = "next-option"

	// Docker panel. Uppercase is always the whole-stack version.
	PreviousService Action = "previous-service"
	NextService     Action = "next-service"
	UpService       Action = "up-service"
	StopService     Action = "stop-service"
	RestartService  Action = "restart-service"
	RebuildService  Action = "rebuild-service"
	LogAsCell       Action = "log-as-cell"
	ShellAsCell     Action = "shell-as-cell"
	UpStack         Action = "up-stack"
	StopStack       Action = "stop-stack"
	RestartStack    Action = "restart-stack"
)

// Shortcut is one line of the map: the key, what it does and the help text.
type Shortcut struct {
	Key    string
	Action Action
	Help   string
	// Group groups in the help the keys that are the same idea (the
	// arrows, the numbers). Empty when the key explains itself.
	Group string
	// Footer marks the handful of keys that fit in the bottom bar, and
	// Short is how they show up there.
	Footer bool
	Short  string
}

// shortcuts is the whole keyboard. One key, one meaning, in any screen.
var shortcuts = map[Mode][]Shortcut{
	Browse: {
		// Navigation is exclusively directional: no letter moves through the
		// grid.
		{Key: "left", Action: CellPrevious, Help: "the arrow moves to the cell that's literally on that side of the grid, crossing projects", Group: "↑↓←→ cell", Footer: true, Short: "↑↓←→ cell"},
		{Key: "right", Action: CellNext, Help: "cell to the right", Group: "↑↓←→ cell"},
		{Key: "up", Action: CellUp, Help: "cell in the row above", Group: "↑↓←→ cell"},
		{Key: "down", Action: CellDown, Help: "cell in the row below", Group: "↑↓←→ cell"},
		{Key: "space", Action: SkipToAlert, Help: "jumps to the next cell asking for attention, crossing projects"},
		{Key: "tab", Action: NextTab, Help: "switches the cell's tab: claude, cursor, shell", Group: "tab ⇥ tab", Footer: true, Short: "tab tab"},
		{Key: "shift+tab", Action: PreviousTab, Help: "previous tab", Group: "tab ⇥ tab"},

		{Key: "enter", Action: EnterType, Help: "enters TYPE on the focused cell", Footer: true, Short: "↵ type"},
		{Key: "o", Action: Fullscreen, Help: "focused cell in fullscreen (toggles)"},
		{Key: "v", Action: SwitchScreen, Help: "switches between grid and list", Footer: true, Short: "v list"},

		{Key: "n", Action: Create, Help: "create — asks for the project, then the cell", Footer: true, Short: "n create"},
		{Key: "r", Action: Resume, Help: "resumes a stopped cell, or brings a dead cell back up"},
		{Key: "D", Action: Kill, Help: "kills the focused cell — always confirms"},

		{Key: "ctrl+r", Action: Rename, Help: "asks the agent to pick a name for the conversation and adopts that name for the cell"},

		{Key: "p", Action: Prompt, Help: "sends a prompt to the focused cell without entering it"},
		{Key: "d", Action: OpenDocker, Help: "opens the focused project's Docker panel", Footer: true, Short: "d docker"},
		{Key: "ctrl+e", Action: OpenEditor, Help: "opens the project's directory in the configured IDE"},

		{Key: "/", Action: SearchTerm, Help: "searches the focused cell's history"},
		{Key: "pgup", Action: ScrollPageUp, Help: "scrolls the focused cell's history a page at a time", Group: "pgup/pgdn history"},
		{Key: "pgdown", Action: ScrollPageDown, Help: "scrolls a page down", Group: "pgup/pgdn history"},
		{Key: "home", Action: ScrollTop, Help: "jumps to the top of the focused cell's history"},
		{Key: "end", Action: ScrollBottom, Help: "jumps back to the live end of the focused cell's history"},
		{Key: "esc", Action: Back, Help: "exits scrolling and closes whatever is open"},

		{Key: "?", Action: Help, Help: "help", Footer: true, Short: "? help"},
		{Key: "q", Action: Close, Help: "closes the screen — the engine keeps running"},
	},
	Type: {
		// The one reserved key. Everything else goes to the cell.
		{Key: "ctrl+l", Action: ExitType, Help: "gives back the keyboard to the application", Footer: true, Short: "ctrl-l gives back the keyboard"},
	},
	Form: {
		{Key: "enter", Action: Confirm, Help: "confirms", Footer: true, Short: "↵ confirms"},
		{Key: "esc", Action: Cancel, Help: "cancels", Footer: true, Short: "esc cancels"},
		{Key: "tab", Action: Complete, Help: "completes the path"},
		{Key: "up", Action: PreviousField, Help: "moves between fields", Group: "↑↓ field"},
		{Key: "down", Action: NextField, Help: "next field", Group: "↑↓ field"},
		{Key: "left", Action: PreviousOpt, Help: "moves between the field's options", Group: "←→ option"},
		{Key: "right", Action: NextOpt, Help: "next option", Group: "←→ option"},
	},
	DockerPanel: {
		{Key: "up", Action: PreviousService, Help: "picks the service — picking doesn't execute anything", Group: "↑↓ pick service", Footer: true, Short: "↑↓ service"},
		{Key: "down", Action: NextService, Help: "next service", Group: "↑↓ pick service"},
		{Key: "u", Action: UpService, Help: "brings up the picked service", Footer: true, Short: "u up"},
		{Key: "s", Action: StopService, Help: "stops the picked service", Footer: true, Short: "s stop"},
		{Key: "r", Action: RestartService, Help: "restarts the picked service", Footer: true, Short: "r restart"},
		{Key: "b", Action: RebuildService, Help: "rebuilds the picked service"},
		{Key: "l", Action: LogAsCell, Help: "opens the service's log as a cell", Footer: true, Short: "l log becomes cell"},
		{Key: "a", Action: ShellAsCell, Help: "attaches a shell inside the picked service's container", Footer: true, Short: "a shell becomes cell"},
		{Key: "U", Action: UpStack, Help: "brings up the whole stack"},
		{Key: "S", Action: StopStack, Help: "stops the whole stack"},
		{Key: "R", Action: RestartStack, Help: "restarts the whole stack"},
		{Key: "esc", Action: Back, Help: "closes the panel and gives back the keyboard", Footer: true, Short: "esc closes"},
	},
}

func init() {
	// Numbers go straight to project N. They stay in the map like any other
	// key, grouped into a single help line.
	for n := 1; n <= 9; n++ {
		shortcuts[Browse] = append(shortcuts[Browse], Shortcut{
			Key:    strconv.Itoa(n),
			Action: GoToProject,
			Help:   "goes straight to project N",
			Group:  "1…9 project N",
		})
	}
}

// Lookup translates a key in the current mode. What's not in the map means
// nothing — in TYPE and in the form, it goes whole to whoever holds the
// keyboard.
func Lookup(mode Mode, key string) Action {
	for _, shortcut := range shortcuts[mode] {
		if shortcut.Key == key {
			return shortcut.Action
		}
	}
	return None
}

// Shortcuts is the mode's map, for the help and the footer to draw.
func Shortcuts(mode Mode) []Shortcut {
	copied := make([]Shortcut, len(shortcuts[mode]))
	copy(copied, shortcuts[mode])
	return copied
}

// Modes lists every mode that has its own keyboard.
func Modes() []Mode {
	return []Mode{Browse, Type, Form, DockerPanel}
}

// Line is how a key shows up in the help: already grouped when it's part of
// a family.
type Line struct {
	Keys string
	Help string
}

// HelpLines builds a mode's help, one line per idea.
func HelpLines(mode Mode) []Line {
	var lines []Line
	seen := map[string]bool{}
	for _, shortcut := range shortcuts[mode] {
		if shortcut.Group != "" {
			if seen[shortcut.Group] {
				continue
			}
			seen[shortcut.Group] = true
			lines = append(lines, Line{Keys: shortcut.Group, Help: shortcut.Help})
			continue
		}
		lines = append(lines, Line{Keys: display(shortcut.Key), Help: shortcut.Help})
	}
	return lines
}

// display swaps the key's internal name for the symbol the user sees.
func display(key string) string {
	switch key {
	case "enter":
		return "↵"
	case "space":
		return "space"
	case "up":
		return "↑"
	case "down":
		return "↓"
	case "left":
		return "←"
	case "right":
		return "→"
	}
	return key
}
