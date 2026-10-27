package cell

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAwayIsAConversationOffItsOwnScreen — Claude Code's list of agents is
// what the cell shows after ← on an empty prompt. A conversation that merely
// quotes the list's words isn't away.
func TestAwayIsAConversationOffItsOwnScreen(t *testing.T) {
	agentView := "Your conversation moved to the background — enter opens it · esc returns to it\n" +
		"Needs input\n ✻ auth   done   3s\n" +
		"────\n❯ describe a task for a new session\n────\n" +
		"  enter to return · space to reply"
	if !anyLineStarts(agentView, claudeMarkers.Away) {
		t.Fatal("the list of agents should read as a conversation in the background")
	}
	// The notice at the top goes away; the box under the list stays.
	if !anyLineStarts("Needs input\n❯ describe a task for a new session", claudeMarkers.Away) {
		t.Fatal("the list without its opening notice is still the list")
	}
	quoted := "● The marker is \"❯ describe a task for a new session\".\n" +
		"  + Away: []string{\"Your conversation moved to the background\"}\n❯ "
	if anyLineStarts(quoted, claudeMarkers.Away) {
		t.Fatal("a conversation that quotes the words was read as away")
	}
}

// TestPromptIsRefusedWhileTheListIsOnScreen — the conversation sent to the
// background keeps its mod and its link, so a prompt would be taken and run
// where nobody is looking. The cell reads the list off its own screen and
// says what to press instead.
func TestPromptIsRefusedWhileTheListIsOnScreen(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "agent-on-the-list")
	body := "#!/bin/sh\nprintf 'Your conversation moved to the background — enter opens it\\r\\n'\nsleep 30\n"
	if err := os.WriteFile(program, []byte(body), 0o755); err != nil {
		t.Fatalf("prepare agent: %v", err)
	}
	session := &Session{}
	if err := session.Spawn(Config{
		ID: "c1", Directory: dir, Columns: 80, Lines: 10,
		Profiles: map[string]Profile{"claude": {Program: program}},
	}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer session.Kill()

	taken := 0
	if _, err := session.Attach("claude", func(delivery Delivery) error {
		taken++
		return session.Delivered("claude", delivery.ID, true)
	}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	for deadline := time.Now().Add(3 * time.Second); session.State() != Background; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the list on screen never read as background, state is %s", session.State())
		}
	}
	if err := session.Prompt("cover the mobile menu"); !errors.Is(err, errNotOnScreen) {
		t.Fatalf("a prompt for a conversation off screen should be refused, got %v", err)
	}
	if taken != 0 {
		t.Fatal("the prompt reached the session in the background anyway")
	}
}

// TestResumeTakesTheConversationBackFromTheBackground — a conversation sent
// to the background lives in a session Claude Code's service hosts, and
// resuming it while it runs there is refused. The cell stops that session
// first, by the short id the service knows it by; a conversation that never
// left the cell costs no extra process.
func TestResumeTakesTheConversationBackFromTheBackground(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	log := filepath.Join(t.TempDir(), "argv.txt")
	program := filepath.Join(dir, "claude")
	body := "#!/bin/sh\necho \"$@\" >> " + log + "\n[ \"$1\" = stop ] && exit 0\nsleep 30\n"
	if err := os.WriteFile(program, []byte(body), 0o755); err != nil {
		t.Fatalf("prepare agent: %v", err)
	}
	const hosted, plain = "d01ea55f-df74-49cb-bb35-e6f238303680", "99bb2e3a-9b13-4271-845d-adb4fdc1f4a8"
	folder := claudeFolder(filepath.Join(home, ".claude"), dir)
	for _, path := range []string{folder, filepath.Join(home, ".claude", "jobs", hosted[:8])} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("prepare disk: %v", err)
		}
	}
	for _, conversation := range []string{hosted, plain} {
		if err := os.WriteFile(filepath.Join(folder, conversation+".jsonl"), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("prepare transcript: %v", err)
		}
		agent := &Claude{}
		if err := agent.Spawn(Config{
			ID: "c1", Directory: dir, Columns: 80, Lines: 10, Conversation: conversation,
			Profiles: map[string]Profile{"claude": {Program: program}},
		}); err != nil {
			t.Fatalf("spawn: %v", err)
		}
		defer agent.Kill()
	}
	var ran string
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		content, _ := os.ReadFile(log)
		if ran = string(content); strings.Count(ran, "\n") == 3 {
			break
		}
	}
	want := "stop d01ea55f\n--resume " + hosted + "\n--resume " + plain + "\n"
	if ran != want {
		t.Fatalf("the agent was run as\n%swant\n%s", ran, want)
	}
}
