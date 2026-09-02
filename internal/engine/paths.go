package engine

import (
	"os"
	"path/filepath"
)

// StateDir is where the engine keeps the desired state and the histories.
func StateDir() string {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "tesseract")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "tesseract")
	}
	return filepath.Join(home, ".local", "state", "tesseract")
}

// SocketPath is the meeting point between the screen and the engine.
func SocketPath() string {
	if base := os.Getenv("XDG_RUNTIME_DIR"); base != "" {
		return filepath.Join(base, "tesseract", "engine.sock")
	}
	return filepath.Join(StateDir(), "engine.sock")
}

// StatePath is the desired-state file.
func StatePath(stateDir string) string {
	return filepath.Join(stateDir, "state.json")
}

// HistoryPath is a cell's history file.
func HistoryPath(stateDir, cellID string) string {
	return filepath.Join(stateDir, "history", cellID+".log")
}

// HistoryDir is where every cell's recording lives.
func HistoryDir(stateDir string) string {
	return filepath.Join(stateDir, "history")
}

// HistoryFiles is everything a cell recorded: its own file plus one per tab,
// on a cell that has them. Killing a cell used to take only the first one,
// and every tab's conversation stayed on disk forever.
func HistoryFiles(stateDir, cellID string) []string {
	found, err := filepath.Glob(filepath.Join(HistoryDir(stateDir), cellID+"*.log"))
	if err != nil {
		return nil
	}
	return found
}
