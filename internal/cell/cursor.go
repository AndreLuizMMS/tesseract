package cell

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func init() {
	Register(Descriptor{
		Type:            "cursor",
		Order:           20,
		AcceptsPrompt:   true,
		HasConversation: true,
	}, func() Cell { return &Cursor{} })
}

// cursorMarkers: the Cursor CLI doesn't announce work with a fixed text, so
// what's left is the screen changing. The blocking question, that it does
// write.
var cursorMarkers = Markers{
	Question: []string{"Allow this command?", "Run this command?"},
}

// Cursor is the Cursor CLI running in the project's directory.
type Cursor struct {
	Agent
}

func (c *Cursor) Spawn(cfg Config) error {
	profile := cfg.Profiles["cursor"]
	program := profile.Program
	if program == "" {
		program = "cursor-agent"
	}
	c.renameCommand = profile.RenameCommand
	if c.renameCommand == "" {
		c.renameCommand = "/rename-chat"
	}
	c.readName = cursorConversationName
	c.findConversation = newCursorConversation

	args := append([]string{}, profile.Args...)
	// Same rule as Claude: only resume what exists on the agent's disk.
	if cfg.Conversation != "" && cursorHasConversation(cfg.Conversation) {
		args = append(args, "--resume", cfg.Conversation)
	}
	return c.spawn(cfg, profile, program, args, cursorMarkers)
}

// cursorHasConversation says whether the conversation still exists on the
// agent's disk.
func cursorHasConversation(conversation string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	found, err := filepath.Glob(filepath.Join(home, ".cursor", "chats", "*", conversation, "meta.json"))
	return err == nil && len(found) > 0
}

// cursorConversationEntry is what the Cursor CLI keeps about each
// conversation.
type cursorConversationEntry struct {
	Cwd       string `json:"cwd"`
	Title     string `json:"title"`
	CreatedAt int64  `json:"createdAtMs"`
	UpdatedAt int64  `json:"updatedAtMs"`
	path      string
	id        string
}

// cursorEntries reads the conversations Cursor has for a directory.
func cursorEntries(directory string) []cursorConversationEntry {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	// Cursor keeps conversations under a hash of the workspace that can't be
	// recomputed from here, so the search is over all of them and the
	// filter is each one's own directory field.
	metas, err := filepath.Glob(filepath.Join(home, ".cursor", "chats", "*", "*", "meta.json"))
	if err != nil {
		return nil
	}
	var entries []cursorConversationEntry
	for _, meta := range metas {
		content, err := os.ReadFile(meta)
		if err != nil {
			continue
		}
		var entry cursorConversationEntry
		if err := json.Unmarshal(content, &entry); err != nil {
			continue
		}
		if entry.Cwd != directory {
			continue
		}
		entry.path = meta
		entry.id = filepath.Base(filepath.Dir(meta))
		entries = append(entries, entry)
	}
	return entries
}

// newCursorConversation figures out which conversation Cursor opened for
// this cell: the one born after the cell came up.
func newCursorConversation(directory string, since time.Time) string {
	limit := since.Add(-2 * time.Second).UnixMilli()
	var chosen string
	var newest int64
	for _, entry := range cursorEntries(directory) {
		if entry.CreatedAt < limit || entry.CreatedAt < newest {
			continue
		}
		newest, chosen = entry.CreatedAt, entry.id
	}
	return chosen
}

// cursorConversationName reads the name Cursor gave the conversation.
func cursorConversationName(directory, conversation string) (string, error) {
	entries := cursorEntries(directory)
	if len(entries) == 0 {
		return "", fmt.Errorf("Cursor hasn't saved anything for this folder yet")
	}
	if conversation != "" {
		for _, entry := range entries {
			if entry.id == conversation {
				return entry.Title, nil
			}
		}
		return "", fmt.Errorf("the conversation doesn't have a name yet")
	}
	var chosen cursorConversationEntry
	for _, entry := range entries {
		if entry.UpdatedAt >= chosen.UpdatedAt {
			chosen = entry
		}
	}
	return chosen.Title, nil
}
