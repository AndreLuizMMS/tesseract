package cell

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	//
	// "thinking" covers the long pause between two tool calls: while the
	// model reasons, the bar drops the token counter and says only that
	// it's thinking, and without this the cell reads a mid-turn pause as an
	// answer and notifies for every step of the work.
	Working: []string{"esc to interrupt", "tokens", "thought for", "thinking"},
	Question: []string{
		"No, and tell Claude what to do differently",
		// The permission question changes with the tool — proceed, make
		// this edit, create the file. Only the opening survives all of
		// them, and an unrecognized question would end the turn as if the
		// answer had come.
		"Do you want to",
		"Yes, I trust this folder",
	},
	// With Tesseract's own skin loaded (mods/tesseract-skin) the spinner and
	// the permission line are redrawn with the mosaic's glyphs, and none of
	// the texts above survive. Inside a cell the skin writes no state word,
	// so the glyph that opens the line is the marker: ▸ opens the spinner and
	// a tool call that's running or waiting its turn, ⏵ the one call that's
	// asking. Only a line that opens with it counts — the glyph in the middle
	// of a sentence is somebody quoting it.
	WorkingLines:  []string{"▸ "},
	QuestionLines: []string{"⏵ "},
	// ← on an empty prompt sends the conversation to a background session and
	// leaves the cell on Claude Code's list of agents. The first text is the
	// notice it opens with, the second the placeholder of the box under the
	// list (Claude Code 2.1.295). Only a line that opens with one counts: a
	// conversation that quotes these words isn't away.
	Away: []string{"Your conversation moved to the background", "❯ describe a task for a new session"},
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
		reclaim(program, cfg.Conversation)
		args = append(args, "--resume", cfg.Conversation)
	} else {
		args = append(args, "--session-id", cfg.Conversation)
	}

	return c.spawn(cfg, profile, program, args, claudeMarkers)
}

// reclaimWait bounds the one command reclaim runs. Measured, it takes 0.7 s.
const reclaimWait = 5 * time.Second

// reclaim takes a conversation back from Claude Code's background service
// before it's resumed. ← on an empty prompt moves the conversation into a
// session the service hosts, and that session outlives the cell's process:
// resuming it while it runs there is refused ("This session is running in
// the background"), and the cell would come back dead. Stopping it keeps the
// conversation and lets this process be the one that runs it again.
//
// The service keeps a folder per session it ever hosted, named after the
// first eight characters of the id — the same short id `claude stop` takes.
// No folder is the normal case, and costs one stat.
func reclaim(program, conversation string) {
	home, err := os.UserHomeDir()
	if err != nil || len(conversation) < 8 {
		return
	}
	short := conversation[:8]
	if _, err := os.Stat(filepath.Join(home, ".claude", "jobs", short)); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), reclaimWait)
	defer cancel()
	// A session that already stopped answers the same way; one that can't be
	// stopped shows up as the resume failing, on the cell's own screen.
	_ = exec.CommandContext(ctx, program, "stop", short).Run()
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
		return "", fmt.Errorf("The conversation has no transcript yet. p sends its first prompt.")
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
		return "", fmt.Errorf("The conversation has no transcript yet. p sends its first prompt.")
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
		return "", fmt.Errorf("The conversation has no transcript yet. p sends its first prompt.")
	}
	return chosen, nil
}
