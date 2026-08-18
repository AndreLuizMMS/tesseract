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
	// Editor is the command `ctrl-e` runs in the project's directory: `cursor
	// /path/to/project` opens the IDE there.
	Editor string `json:"editor,omitempty"`
	// Sound and Notify turn the two alerts on and off, independently.
	Sound  bool `json:"sound"`
	Notify bool `json:"notify"`
	// NotifyCommand swaps the detected notifier for another one. The program
	// receives the title and the body as its last two arguments.
	NotifyCommand string `json:"notifyCommand,omitempty"`
	// HistoryLimit is the maximum size of each cell's history, in bytes.
	HistoryLimit int64 `json:"historyLimit,omitempty"`
	// Agents is the profile of each agent type.
	Agents map[string]AgentProfile `json:"agents,omitempty"`
}

// DefaultConfig is what applies when there is no config file — and what
// applies for every field the file doesn't bring.
func DefaultConfig() Config {
	return Config{
		Editor:       "cursor",
		Sound:        true,
		Notify:       true,
		HistoryLimit: history.DefaultCap,
		Agents:       map[string]AgentProfile{},
	}
}

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
	if config.Agents == nil {
		config.Agents = map[string]AgentProfile{}
	}
	return config, nil
}
