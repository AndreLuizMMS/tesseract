package cell

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	// The account is chosen at birth and holds for the process's life: a
	// conversation only changes accounts by coming back up on the other one.
	account := nativeAccount()
	if profile.Account != nil {
		if chosen := profile.Account(); chosen != "" {
			account = chosen
		}
	}
	c.account = account
	c.readName = func(directory, conversation string) (string, error) {
		return claudeConversationName(account, directory, conversation)
	}
	var environment []string
	if account != nativeAccount() {
		environment = []string{"CLAUDE_CONFIG_DIR=" + account}
	}

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
	carryTrust(account, cfg.Directory)
	adoptTranscript(profile.Accounts, account, cfg.Directory, cfg.Conversation)
	if hasTranscript(account, cfg.Directory, cfg.Conversation) {
		reclaim(program, account, cfg.Conversation, environment)
		args = append(args, "--resume", cfg.Conversation)
	} else {
		args = append(args, "--session-id", cfg.Conversation)
	}
	if cfg.Prompt != "" {
		args = append(args, cfg.Prompt)
	}

	return c.spawn(cfg, profile, program, args, claudeMarkers, environment...)
}

// nativeAccount is the folder Claude Code keeps everything in when nobody
// points it elsewhere: the login, the transcripts, the background sessions.
func nativeAccount() string {
	if folder := os.Getenv("CLAUDE_CONFIG_DIR"); folder != "" {
		return folder
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude")
}

// accountState is the file where an account keeps its answers per folder:
// next to Claude Code's own folder (~/.claude.json), inside any other.
func accountState(account string) string {
	if account == nativeAccount() && os.Getenv("CLAUDE_CONFIG_DIR") == "" {
		return filepath.Join(filepath.Dir(account), ".claude.json")
	}
	return filepath.Join(account, ".claude.json")
}

// carryTrust answers "do you trust this folder?" on another account the way
// it was already answered on Claude Code's own. A conversation moved there
// because the first account hit its limit would otherwise come back up
// stuck on that question, with nobody looking. A folder never trusted on
// the native account is left for the user to answer.
//
// ponytail: read, change and rename, with no lock — a Claude on that account
// rewriting the file in the same instant can lose this answer, and the
// question shows up once more. Claude Code's own lock is the upgrade.
func carryTrust(account, directory string) {
	if account == nativeAccount() {
		return
	}
	directory = strings.TrimSuffix(directory, "/")
	native, _ := readState(accountState(nativeAccount()))
	if !trusted(native, directory) {
		return
	}
	path := accountState(account)
	state, err := readState(path)
	if err != nil || trusted(state, directory) {
		return
	}
	projects, _ := state["projects"].(map[string]any)
	if projects == nil {
		projects = map[string]any{}
	}
	project, _ := projects[directory].(map[string]any)
	if project == nil {
		project = map[string]any{}
	}
	project["hasTrustDialogAccepted"] = true
	projects[directory] = project
	state["projects"] = projects

	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil || os.MkdirAll(account, 0o700) != nil {
		return
	}
	partial := path + ".partial"
	if os.WriteFile(partial, content, 0o600) != nil {
		_ = os.Remove(partial)
		return
	}
	_ = os.Rename(partial, path)
}

// readState reads an account's state file. A missing file is an empty one;
// one that doesn't parse is an error, so it's never written over. Numbers
// stay as written: the file carries ids too big for a float.
func readState(path string) (map[string]any, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	state := map[string]any{}
	if err := decoder.Decode(&state); err != nil {
		return nil, err
	}
	return state, nil
}

// trusted says the state already answered yes for the folder.
func trusted(state map[string]any, directory string) bool {
	projects, _ := state["projects"].(map[string]any)
	project, _ := projects[directory].(map[string]any)
	accepted, _ := project["hasTrustDialogAccepted"].(bool)
	return accepted
}

// adoptTranscript brings the newest copy of the conversation into the account
// it's about to run on. Each account keeps its own copy of every conversation
// it ran, and they part ways the moment one hits its limit: without this, a
// conversation that moved to the second account would come back to the first
// one where it left it, short of everything said since.
func adoptTranscript(accounts []string, account, directory, conversation string) {
	if conversation == "" {
		return
	}
	target := filepath.Join(claudeFolder(account, directory), conversation+".jsonl")
	newest, newestAt := target, time.Time{}
	if info, err := os.Stat(target); err == nil {
		newestAt = info.ModTime()
	}
	for _, other := range accounts {
		path := filepath.Join(claudeFolder(other, directory), conversation+".jsonl")
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.ModTime().After(newestAt) {
			newest, newestAt = path, info.ModTime()
		}
	}
	if newest == target {
		return
	}
	content, err := os.ReadFile(newest)
	if err != nil || os.MkdirAll(filepath.Dir(target), 0o700) != nil {
		return
	}
	// Written aside and renamed: a copy cut in half would be the newest one on
	// disk, and every account would adopt it from then on.
	partial := target + ".partial"
	if os.WriteFile(partial, content, 0o600) != nil {
		_ = os.Remove(partial)
		return
	}
	_ = os.Rename(partial, target)
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
func reclaim(program, account, conversation string, environment []string) {
	if len(conversation) < 8 {
		return
	}
	short := conversation[:8]
	if _, err := os.Stat(filepath.Join(account, "jobs", short)); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), reclaimWait)
	defer cancel()
	// A session that already stopped answers the same way; one that can't be
	// stopped shows up as the resume failing, on the cell's own screen.
	stop := exec.CommandContext(ctx, program, "stop", short)
	stop.Env = append(os.Environ(), environment...)
	_ = stop.Run()
}

// hasTranscript says whether the conversation already exists on the agent's
// disk.
func hasTranscript(account, directory, conversation string) bool {
	if conversation == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(claudeFolder(account, directory), conversation+".jsonl"))
	return err == nil && !info.IsDir()
}

// claudeFolder is where an account keeps the transcripts of a directory's
// conversations: the path becomes the folder name with slashes swapped for
// dashes.
func claudeFolder(account, directory string) string {
	alias := strings.ReplaceAll(strings.TrimSuffix(directory, "/"), "/", "-")
	alias = strings.ReplaceAll(alias, ".", "-")
	return filepath.Join(account, "projects", alias)
}

// claudeConversationName reads the conversation's name from the transcript.
// A hand-chosen name takes priority over the title the agent generated on
// its own.
func claudeConversationName(account, directory, conversation string) (string, error) {
	folder := claudeFolder(account, directory)
	file := filepath.Join(folder, conversation+".jsonl")
	if conversation == "" {
		var err error
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
