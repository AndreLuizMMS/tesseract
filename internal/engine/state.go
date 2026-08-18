package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// SavedCell is the saved intent of a cell: what it is, not the process it was
// running. The process dies; this stays.
type SavedCell struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Target string `json:"target,omitempty"`
	// Tab is which tab was showing, on cells that have several.
	Tab string `json:"tab,omitempty"`
	// Conversations is each tab's conversation identity, to reattach without
	// losing anything. Conversation is the old format, from when the cell had
	// only one.
	Conversations map[string]string `json:"conversations,omitempty"`
	Conversation  string            `json:"conversation,omitempty"`
}

// SavedProject is one column of the mosaic saved to disk.
type SavedProject struct {
	ID    string      `json:"id"`
	Path  string      `json:"path"`
	Color int         `json:"color"`
	Cells []SavedCell `json:"cells"`
}

// DesiredState is the whole grid as the user left it.
type DesiredState struct {
	Projects []SavedProject `json:"projects"`
	Screen   string         `json:"screen,omitempty"` // mosaico or lista
}

// Save writes the desired state atomically: writes alongside and renames on
// top, so a crash mid-write never leaves a half-written file.
func Save(path string, state DesiredState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')

	tempFile, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".writing-*")
	if err != nil {
		return err
	}
	tempName := tempFile.Name()
	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		os.Remove(tempName)
		return err
	}
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		os.Remove(tempName)
		return err
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempName)
		return err
	}
	if err := os.Chmod(tempName, 0o644); err != nil {
		os.Remove(tempName)
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		os.Remove(tempName)
		return err
	}
	return nil
}

// Load reads the desired state. When the file is corrupted, it loads what it
// can, keeps the original under a suffix, and returns the error for the
// screen to warn about.
func Load(path string) (DesiredState, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DesiredState{}, nil
	}
	if err != nil {
		return DesiredState{}, err
	}

	var raw struct {
		Projects []json.RawMessage `json:"projects"`
		Screen   string            `json:"screen"`
	}
	if err := json.Unmarshal(content, &raw); err != nil {
		preserved, preserveErr := preserve(path)
		if preserveErr != nil {
			return DesiredState{}, fmt.Errorf("unreadable state and could not preserve it: %w", preserveErr)
		}
		return DesiredState{}, fmt.Errorf("unreadable state, the previous file is at %s: %w", preserved, err)
	}

	state := DesiredState{Screen: raw.Screen}
	lost := 0
	for _, rawProject := range raw.Projects {
		var project SavedProject
		if err := json.Unmarshal(rawProject, &project); err != nil || project.Path == "" {
			lost++
			continue
		}
		state.Projects = append(state.Projects, project)
	}
	if lost > 0 {
		preserved, preserveErr := preserve(path)
		if preserveErr != nil {
			return state, fmt.Errorf("%d unreadable project(s) and could not preserve the file: %w", lost, preserveErr)
		}
		return state, fmt.Errorf("%d unreadable project(s) were discarded, the previous file is at %s", lost, preserved)
	}
	return state, nil
}

// preserve keeps a copy of the problem file under a suffix, without
// overwriting earlier preservations.
func preserve(path string) (string, error) {
	for i := 0; ; i++ {
		dest := path + ".corrupted"
		if i > 0 {
			dest += "-" + strconv.Itoa(i)
		}
		if _, err := os.Stat(dest); err == nil {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return "", err
		}
		return dest, nil
	}
}
