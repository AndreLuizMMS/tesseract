package engine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/andreluiz/tesseract/internal/engine/history"
)

// AgentProfile is how an agent type starts up on this machine.
type AgentProfile struct {
	Program       string   `json:"program,omitempty"`
	Args          []string `json:"args,omitempty"`
	RenameCommand string   `json:"renameCommand,omitempty"`
	// WorkMarkers and QuestionMarkers change what the cell looks for in the
	// agent's screen to know what turn state it's in. When empty, the type's
	// own markers apply — which is the normal case.
	WorkMarkers     []string `json:"workMarkers,omitempty"`
	QuestionMarkers []string `json:"questionMarkers,omitempty"`
}

// Config is what the user can change without touching code.
type Config struct {
	// Sound and Notify turn the two alerts on and off, independently.
	Sound  bool `json:"sound"`
	Notify bool `json:"notify"`
	// NotifyCommand swaps the detected notifier for another one. The program
	// receives the title and the body as its last two arguments.
	NotifyCommand string `json:"notifyCommand,omitempty"`
	// HistoryLimit is the maximum size of each cell's history, in bytes.
	HistoryLimit int64 `json:"historyLimit,omitempty"`
	// ScrollbackLines is how many lines each cell keeps above what's
	// showing. Every line stays in memory fully styled: ten thousand of
	// them cost a hundred megabytes per cell, and a project with several
	// tabs open pays that many times over. Scrolling further back than this
	// is what the recorded history is for.
	ScrollbackLines int `json:"scrollbackLines,omitempty"`
	// SleepAfterMinutes is how long an agent tab out of view can stay
	// silent before its process is put to sleep. The conversation isn't
	// lost — coming back to the tab reattaches it. Zero turns it off.
	SleepAfterMinutes int `json:"sleepAfterMinutes,omitempty"`
	// Mods loads Tesseract's mods into every Claude Code cell: it's through
	// them that the agent reports its own turn, conversation and usage.
	// Off, the cell goes back to being read from the outside.
	Mods bool `json:"mods"`
	// ClaudeTheme is the theme Claude Code is started with inside a cell.
	// The default draws with the terminal's own colors, so the agent speaks
	// the grid's palette; it holds for the cell's session only. Empty leaves
	// the user's own theme alone.
	ClaudeTheme string `json:"claudeTheme"`
	// DockerDeny starts every Claude Code cell with native deny rules for the
	// Docker commands that delete data. It's a second layer under the
	// tesseract-docker mod's guard: the rules hold even when the mod didn't
	// load, and in a session that skips permission prompts.
	DockerDeny bool `json:"dockerDeny"`
	// Roster loads the tesseract-roster mod inside cells. Off by default: the
	// mosaic already says who's calling.
	Roster bool `json:"roster"`
	// ClaudeAccounts are the config folders of the Claude accounts on this
	// machine, primary first ("~/.claude", "~/.claude-2"). A cell is born on
	// the first one that isn't at its usage limit, and a conversation that
	// hits the limit comes back up on the next one, where it left off. Empty,
	// every cell runs on Claude Code's own folder.
	ClaudeAccounts []string `json:"claudeAccounts,omitempty"`
	// Agents is the profile of each agent type.
	Agents map[string]AgentProfile `json:"agents,omitempty"`
}

// DefaultConfig is what applies when there is no config file — and what
// applies for every field the file doesn't bring.
func DefaultConfig() Config {
	return Config{
		Sound:             true,
		Notify:            true,
		HistoryLimit:      history.DefaultCap,
		ScrollbackLines:   DefaultScrollback,
		SleepAfterMinutes: DefaultSleepAfterMinutes,
		Mods:              true,
		ClaudeTheme:       "dark-ansi",
		DockerDeny:        true,
		Agents:            map[string]AgentProfile{},
	}
}

// DefaultScrollback is how far back a cell can be scrolled without touching
// the recorded history. Two thousand lines is eighty screens of reading and
// costs a quarter of what the emulator's own default costs.
const DefaultScrollback = 2000

// DefaultSleepAfterMinutes is how long an agent tab nobody is looking at can
// stay silent before its process is released.
const DefaultSleepAfterMinutes = 15

// ConfigPath is where the file lives.
func ConfigPath() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "tesseract", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "tesseract-config.json")
	}
	return filepath.Join(home, ".config", "tesseract", "config.json")
}

// LoadConfig reads the config. A missing file is not an error: the default
// applies. An unreadable file is an error, because staying silent would mean
// ignoring what the user wrote.
func LoadConfig(path string) (Config, error) {
	config := DefaultConfig()
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(content, &config); err != nil {
		return DefaultConfig(), err
	}
	if config.HistoryLimit <= 0 {
		config.HistoryLimit = history.DefaultCap
	}
	if config.ScrollbackLines <= 0 {
		config.ScrollbackLines = DefaultScrollback
	}
	if config.Agents == nil {
		config.Agents = map[string]AgentProfile{}
	}
	return config, nil
}
