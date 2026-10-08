package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// The link is the engine's conversation with the mod that runs inside each
// Claude Code cell. Before it, everything the engine knew about an agent
// came from reading its screen; with it, the agent says so itself. The
// screen reading doesn't go away — it's what's left when there's no mod: an
// older Claude Code, a mod that failed to load, a cursor cell.

// detailLimit caps what a report adds to a notice. A toast shows a line or
// two; the rest of a long command is noise.
const detailLimit = 160

// conversationID is the shape of a Claude Code session id. The id becomes a
// file name when the conversation's title is read, so nothing else gets in.
var conversationID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// reportedStates is what a mod may say about its turn. A session that ended
// has nothing waiting: the process going away is what the cell shows next.
var reportedStates = map[string]cell.State{
	"working":  cell.Working,
	"answered": cell.Replied,
	"approve":  cell.Approve,
	"ended":    cell.Working,
}

// Report takes what a cell's mod says about its own session. Everything
// here came from outside the engine, so everything is checked before it
// touches a cell.
func (e *Engine) Report(report protocol.Report) error {
	c := e.findCell(report.Cell)
	if c == nil || c.proc == nil {
		return fmt.Errorf("cell %s does not exist", report.Cell)
	}

	switch report.Kind {
	case "quota":
		return reportQuota(report.Percent, report.ResetsAt)
	case "title":
		// The mod says the rename the engine asked for has run. One nobody
		// asked for isn't adopted: the cell's label is the user's.
		e.mu.Lock()
		pending := c.pendingRename
		e.mu.Unlock()
		if !pending {
			return nil
		}
		return e.AdoptAgentName(c.id)
	case "session", "state", "delivered":
	default:
		return fmt.Errorf("unknown report kind: %q", report.Kind)
	}

	withLink, ok := c.proc.(cell.WithLink)
	if !ok {
		return fmt.Errorf("cell %s has no agent to report for", c.name)
	}
	if report.Kind == "delivered" {
		return withLink.Delivered(report.Tab, report.Delivery, !report.Refused)
	}
	if report.Kind == "session" {
		if !conversationID.MatchString(report.Session) {
			return fmt.Errorf("invalid session id")
		}
		return withLink.Rebind(report.Tab, report.Session)
	}

	state, known := reportedStates[report.State]
	if !known {
		return fmt.Errorf("unknown state: %q", report.State)
	}
	e.mu.Lock()
	c.detail, c.detailTab = cleanDetail(report.Detail), report.Tab
	e.mu.Unlock()
	return withLink.Signal(report.Tab, state, report.At)
}

// Link attaches a mod's connection to its cell: deliver is how a prompt
// reaches the mod, and the returned function is called when the connection
// drops — which is how the engine learns the mod is gone and goes back to
// reading the screen.
func (e *Engine) Link(cellID, tab string, deliver func(cell.Delivery) error) (func(), error) {
	c := e.findCell(cellID)
	if c == nil || c.proc == nil {
		return nil, fmt.Errorf("cell %s does not exist", cellID)
	}
	withLink, ok := c.proc.(cell.WithLink)
	if !ok {
		return nil, fmt.Errorf("cell %s has no agent to link", c.name)
	}
	return withLink.Attach(tab, deliver)
}

// takeDetail hands over what the mod said about the state being announced,
// once. It only counts for the tab it came from: on a cell with tabs, a
// state read off another agent's screen has nothing to do with it. Called
// with the lock held.
func (c *liveCell) takeDetail() string {
	detail, tab := c.detail, c.detailTab
	c.detail, c.detailTab = "", ""
	if withTabs, ok := c.proc.(cell.WithTabs); ok && tab != withTabs.ActiveTab() {
		return ""
	}
	return detail
}

// withDetail finishes a notice with what the agent itself said, when it
// said anything: "approve: Bash: pnpm migrate".
func withDetail(notice, detail string) string {
	if detail == "" {
		return notice
	}
	return notice + ": " + detail
}

// cleanDetail makes one short line out of whatever the mod sent: a command
// can carry line breaks and escape codes, and a notice has room for neither.
func cleanDetail(raw string) string {
	text := strings.Join(strings.FieldsFunc(raw, func(r rune) bool {
		return r < 0x20 || r == 0x7f || r == ' '
	}), " ")
	if runes := []rune(text); len(runes) > detailLimit {
		text = string(runes[:detailLimit-1]) + "…"
	}
	return text
}

// reportQuota keeps the usage a mod measured, which beats the file the
// user's statusline leaves: it arrives by itself, from any cell, with no
// script to install.
func reportQuota(percent float64, resetsAt string) error {
	if math.IsNaN(percent) || percent < 0 {
		return fmt.Errorf("invalid usage percentage")
	}
	var rollover time.Time
	if resetsAt != "" {
		parsed, err := time.Parse(time.RFC3339, resetsAt)
		if err != nil {
			return fmt.Errorf("invalid reset time")
		}
		rollover = parsed
	}
	reportedQuota.Lock()
	defer reportedQuota.Unlock()
	reportedQuota.percent = min(int(percent+0.5), 100)
	reportedQuota.rollover = rollover
	reportedQuota.at = time.Now()
	return nil
}

// modsRoots are the places the mods can live, in order: wherever the user
// pointed, next to the binary (a clone built with `make build`), and where
// the installer copies them.
func modsRoots() []string {
	var roots []string
	if chosen := os.Getenv("TESSERACT_MODS"); chosen != "" {
		roots = append(roots, chosen)
	}
	if self, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Join(filepath.Dir(self), "mods"))
	}
	if base := os.Getenv("XDG_DATA_HOME"); base != "" {
		roots = append(roots, filepath.Join(base, "tesseract", "mods"))
	} else if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, ".local", "share", "tesseract", "mods"))
	}
	return roots
}

// ModDirs lists the mods every Claude Code cell loads: each tesseract-*
// folder with a manifest, from the first root that has any. A mod that
// isn't there is simply not loaded — the cell works without it.
func ModDirs() []string {
	for _, root := range modsRoots() {
		manifests, _ := filepath.Glob(filepath.Join(root, "tesseract-*", ".claude-plugin", "plugin.json"))
		if len(manifests) == 0 {
			continue
		}
		dirs := make([]string, len(manifests))
		for i, manifest := range manifests {
			dirs[i] = filepath.Dir(filepath.Dir(manifest))
		}
		return dirs
	}
	return nil
}

// claudeArgs is what the engine adds to every Claude Code it brings up: the
// mods, and the theme that draws with the terminal's own colors. Both ride
// on the command line, so they hold for the cell's session and touch nothing
// the user has outside Tesseract.
func (e *Engine) claudeArgs(chosen []string) []string {
	var args []string
	if e.config.Mods {
		for _, dir := range ModDirs() {
			args = append(args, "--plugin-dir", dir)
		}
	}
	// Whoever already passes their own settings keeps them: a second flag
	// would compete with the first.
	if theme := e.config.ClaudeTheme; theme != "" && !slices.Contains(chosen, "--settings") {
		settings, _ := json.Marshal(map[string]string{"theme": theme})
		args = append(args, "--settings", string(settings))
	}
	return args
}
