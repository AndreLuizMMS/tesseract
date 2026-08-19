package cell

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	Register(Descriptor{
		Type:            "claude",
		Order:           10,
		AcceptsPrompt:   true,
		HasConversation: true,
	}, func() Cell { return &Claude{} })
}

// claudeMarkers is what Claude Code writes on its own interface: the first
// while the turn is in progress, the second while it waits for a yes or no.
var claudeMarkers = Markers{
	// The interrupt hint only shows up on a wide cell; in a narrow one the
	// work bar shrinks to the timer and whatever fits after it. What
	// survives every width is the token counter and, before the first
	// token, the time it spent thinking — and neither is always the end of
	// the line, so the marker can't carry the closing paren.
	Working: []string{"esc to interrupt", "tokens", "thought for"},
	Question: []string{
		"No, and tell Claude what to do differently",
		"Do you want to proceed?",
		"1. Yes, I trust this folder",
	},
}

// Claude is Claude Code running in the project's directory.
type Claude struct {
	Agent
}

func (c *Claude) Spawn(cfg Config) error {
	profile := cfg.Profiles["claude"]
	program := profile.Program
	if program == "" {
		program = "claude"
	}
	c.renameCommand = profile.RenameCommand
	if c.renameCommand == "" {
		c.renameCommand = "/rename"
	}
	c.readName = claudeConversationName

	// The conversation has its own identity: with it the agent reattaches
	// where it left off after a WSL crash, and it's how the conversation
	// name is read.
	args := append([]string{}, profile.Args...)
	if cfg.Conversation == "" {
		cfg.Conversation = newConversationID()
		if cfg.OnConversationDiscovered != nil {
			cfg.OnConversationDiscovered(cfg.Tab, cfg.Conversation)
		}
	}
	// Resuming only makes sense if the conversation ever existed on disk. A
	// cell that was born and never received a request has no transcript,
	// and asking to resume it would make the agent die at the start — it
	// starts over with the same identity instead.
	if hasTranscript(cfg.Directory, cfg.Conversation) {
		args = append(args, "--resume", cfg.Conversation)
	} else {
		args = append(args, "--session-id", cfg.Conversation)
	}

	return c.spawn(cfg, profile, program, args, claudeMarkers)
}

// hasTranscript says whether the conversation already exists on the agent's
// disk.
func hasTranscript(directory, conversation string) bool {
	if conversation == "" {
		return false
	}
	folder, err := claudeFolder(directory)
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(folder, conversation+".jsonl"))
	return err == nil && !info.IsDir()
}

// claudeFolder is where Claude Code keeps the transcripts of a directory's
// conversations: the path becomes the folder name with slashes swapped for
// dashes.
func claudeFolder(directory string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	alias := strings.ReplaceAll(strings.TrimSuffix(directory, "/"), "/", "-")
	alias = strings.ReplaceAll(alias, ".", "-")
	return filepath.Join(home, ".claude", "projects", alias), nil
}

// claudeConversationName reads the conversation's name from the transcript.
// A hand-chosen name takes priority over the title the agent generated on
// its own.
func claudeConversationName(directory, conversation string) (string, error) {
	folder, err := claudeFolder(directory)
	if err != nil {
		return "", err
	}
	file := filepath.Join(folder, conversation+".jsonl")
	if conversation == "" {
		file, err = mostRecentTranscript(folder)
		if err != nil {
			return "", err
		}
	}

	opened, err := os.Open(file)
	if err != nil {
		return "", fmt.Errorf("the conversation doesn't have a transcript yet")
	}
	defer opened.Close()

	var chosen, automatic string
	reader := bufio.NewScanner(opened)
	reader.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for reader.Scan() {
		var line struct {
			CustomTitle string `json:"customTitle"`
			AITitle     string `json:"aiTitle"`
		}
		if err := json.Unmarshal(reader.Bytes(), &line); err != nil {
			continue
		}
		if line.CustomTitle != "" {
			chosen = line.CustomTitle
		}
		if line.AITitle != "" {
			automatic = line.AITitle
		}
	}
	if chosen != "" {
		return chosen, nil
	}
	return automatic, nil
}

// mostRecentTranscript serves the conversation whose identity was lost.
func mostRecentTranscript(folder string) (string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return "", fmt.Errorf("the conversation doesn't have a transcript yet")
	}
	var chosen string
	var newest time.Time
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest, chosen = info.ModTime(), filepath.Join(folder, entry.Name())
		}
	}
	if chosen == "" {
		return "", fmt.Errorf("the conversation doesn't have a transcript yet")
	}
	return chosen, nil
}
