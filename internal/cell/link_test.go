package cell

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
)

// screenAskingForApproval is what the screen reading takes for a question.
const screenAskingForApproval = "Bash command\nNo, and tell Claude what to do differently"

// TestSignalBeatsTheScreen is the priority rule: while a mod speaks for the
// turn, nothing written on the screen changes the state.
func TestSignalBeatsTheScreen(t *testing.T) {
	turn := NewTurn(testMarkers)
	turn.Link(true)

	if !turn.Signal(Working, 1) {
		t.Fatal("a linked turn should take the mod's word")
	}
	for range readingsToEnd * 2 {
		if state := turn.Observe(screenAskingForApproval); state != Working {
			t.Fatalf("the screen overrode the mod: %q", state)
		}
	}

	turn.Signal(Approve, 2)
	if state := turn.Observe("esc to interrupt"); state != Approve {
		t.Fatalf("the mod said approve, the cell says %q", state)
	}

	turn.Signal(Replied, 3)
	if state := turn.State(); state != Replied {
		t.Fatalf("the mod said answered, the cell says %q", state)
	}
}

// TestLateReportDoesNotUndoANewerOne — each report travels in its own
// process, and one that loses the race can't put the old state back.
func TestLateReportDoesNotUndoANewerOne(t *testing.T) {
	turn := NewTurn(testMarkers)
	turn.Link(true)
	turn.Signal(Replied, 20)
	if turn.Signal(Working, 10) {
		t.Fatal("a report older than the last one was taken")
	}
	if state := turn.State(); state != Replied {
		t.Fatalf("the straggler changed the state to %q", state)
	}
}

// TestWithoutLinkTheScreenIsInCharge is the fallback: no mod, or a mod that
// went away, and the screen reading works exactly as before.
func TestWithoutLinkTheScreenIsInCharge(t *testing.T) {
	turn := NewTurn(testMarkers)
	if turn.Signal(Approve, 1) {
		t.Fatal("a report with no mod linked was taken")
	}

	turn.Link(true)
	turn.Signal(Working, 2)
	turn.Link(false)
	if state := turn.Observe(screenAskingForApproval); state != Approve {
		t.Fatalf("with the mod gone the screen should decide, got %q", state)
	}
}

// TestAnswerEndsTheWaitForApproval — the mod only speaks again when the
// approved tool finishes; the key that answered the question is what clears
// the state meanwhile.
func TestAnswerEndsTheWaitForApproval(t *testing.T) {
	turn := NewTurn(testMarkers)
	turn.Link(true)
	turn.Signal(Approve, 1)
	turn.Answer()
	if state := turn.State(); state != Working {
		t.Fatalf("after the answer the cell should be working, got %q", state)
	}

	for _, key := range []Keystroke{{Code: vt.KeyEnter}, {Code: vt.KeyEscape}, {Code: '2', Text: "2"}} {
		if !answers(key) {
			t.Errorf("%+v should count as an answer", key)
		}
	}
	for _, key := range []Keystroke{{Code: vt.KeyDown}, {Code: 'y', Text: "y"}, {Paste: "1"}} {
		if answers(key) {
			t.Errorf("%+v shouldn't count as an answer", key)
		}
	}
}

// agentThatLogs records every byte that reaches its keyboard.
func agentThatLogs(t *testing.T, dir, log string) string {
	t.Helper()
	path := filepath.Join(dir, "agent-that-logs")
	body := "#!/bin/sh\nwhile true; do dd bs=1 count=1 2>/dev/null >> " + log + " || exit 0; done\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("prepare agent: %v", err)
	}
	return path
}

// TestPromptGoesThroughTheLink — with a mod attached the prompt is handed to
// it and nothing is typed into the terminal; with the mod gone, or failing,
// the paste is still there.
func TestPromptGoesThroughTheLink(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(t.TempDir(), "received.txt")
	var discovered []string
	session := &Session{}
	err := session.Spawn(Config{
		ID: "c1", Directory: dir, Columns: 60, Lines: 10,
		Profiles:                 map[string]Profile{"claude": {Program: agentThatLogs(t, dir, log)}},
		OnConversationDiscovered: func(tab, id string) { discovered = append(discovered, tab+"="+id) },
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer session.Kill()

	var delivered []string
	fail := false
	detach, err := session.Attach("claude", func(delivery Delivery) error {
		if fail {
			return errors.New("the mod hung up")
		}
		delivered = append(delivered, delivery.Text)
		return session.Delivered("claude", delivery.ID, true)
	})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	if err := session.Prompt("cover the mobile menu"); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if len(delivered) != 1 || delivered[0] != "cover the mobile menu" {
		t.Fatalf("the mod should have received the prompt, got %v", delivered)
	}
	time.Sleep(300 * time.Millisecond)
	if typed, _ := os.ReadFile(log); strings.Contains(string(typed), "cover") {
		t.Fatalf("the prompt was also typed into the terminal: %q", typed)
	}

	if err := session.Signal("claude", Approve, 1); err != nil {
		t.Fatalf("signal: %v", err)
	}
	if state := session.State(); state != Approve {
		t.Fatalf("the mod's state didn't reach the cell: %q", state)
	}

	const conversation = "0b0e7e0c-58a5-4b6e-9f0c-0f6f3c1d2e3f"
	if err := session.Rebind("claude", conversation); err != nil {
		t.Fatalf("rebind: %v", err)
	}
	if got := session.Conversations()["claude"]; got != conversation {
		t.Fatalf("the new conversation wasn't kept: %q", got)
	}
	if last := discovered[len(discovered)-1]; last != "claude="+conversation {
		t.Fatalf("the engine wasn't told about the new conversation: %v", discovered)
	}

	fail = true
	if err := session.Prompt("fallback one"); err != nil {
		t.Fatalf("prompt with a failing mod: %v", err)
	}
	detach()
	if err := session.Prompt("fallback two"); err != nil {
		t.Fatalf("prompt with no mod: %v", err)
	}
	typed := func() string { content, _ := os.ReadFile(log); return string(content) }
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(typed(), "fallback two") {
		time.Sleep(50 * time.Millisecond)
	}
	if got := typed(); !strings.Contains(got, "fallback one") || !strings.Contains(got, "fallback two") {
		t.Fatalf("without the mod the prompt should be pasted, the agent got %q", got)
	}

	if _, err := session.Attach("bash", func(Delivery) error { return nil }); err == nil {
		t.Fatal("a tab with no agent took a link")
	}
}

// linkedAgent brings up a Claude tab whose agent logs its keyboard, with a
// short wait for the mod's answer.
func linkedAgent(t *testing.T) (session *Session, typed func() string) {
	t.Helper()
	patient := ackWait
	ackWait = 150 * time.Millisecond
	t.Cleanup(func() { ackWait = patient })

	dir := t.TempDir()
	log := filepath.Join(t.TempDir(), "received.txt")
	session = &Session{}
	err := session.Spawn(Config{
		ID: "c1", Directory: dir, Columns: 60, Lines: 10,
		Profiles: map[string]Profile{"claude": {Program: agentThatLogs(t, dir, log)}},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	t.Cleanup(func() { _ = session.Kill() })
	return session, func() string { content, _ := os.ReadFile(log); return string(content) }
}

// TestPromptNobodyTookIsTyped — a connection that accepts the bytes isn't a
// mod that took the prompt: the process on the other end may be one that
// shows nothing of the conversation. Without the mod's answer the text goes
// in through the terminal, and a prompt is never lost in silence.
func TestPromptNobodyTookIsTyped(t *testing.T) {
	session, typed := linkedAgent(t)
	var expires int64
	if _, err := session.Attach("claude", func(delivery Delivery) error {
		expires = delivery.Expires
		return nil // written to the socket, and nobody answers
	}); err != nil {
		t.Fatalf("attach: %v", err)
	}

	if err := session.Prompt("cover the mobile menu"); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if left := time.Until(time.UnixMilli(expires)); left > 0 {
		t.Fatalf("the delivery was typed %v before it expired: a slow mod would send it twice", left)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(typed(), "mobile menu") {
		time.Sleep(50 * time.Millisecond)
	}
	if got := typed(); !strings.Contains(got, "cover the mobile menu") {
		t.Fatalf("the unanswered prompt should have been typed, the agent got %q", got)
	}

	// With the agent gone there's no terminal left either, and that's said.
	_ = session.Kill()
	if err := session.Prompt("anyone there"); err == nil {
		t.Fatal("a prompt with nowhere to go reported no error")
	}
}

// TestPromptTheSessionRefusedIsSaid — the mod is there but its session has
// nowhere to put the prompt (the conversation went to the background). The
// text isn't typed into a screen that isn't the conversation's: the user is
// told, with what to press.
func TestPromptTheSessionRefusedIsSaid(t *testing.T) {
	session, typed := linkedAgent(t)
	if _, err := session.Attach("claude", func(delivery Delivery) error {
		return session.Delivered("claude", delivery.ID, false)
	}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	err := session.Prompt("cover the mobile menu")
	if err == nil || !strings.Contains(err.Error(), "↵ enters it") {
		t.Fatalf("a refused prompt should say so and end in what to press, got %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if strings.Contains(typed(), "mobile menu") {
		t.Fatalf("a refused prompt was typed into the terminal anyway: %q", typed())
	}
}

// TestNewestLinkOwnsTheCell — more than one mod connection can be attached
// to a cell. Only the newest gets the prompt; when it drops, the one before
// it takes over, and the mod's word holds until the last one is gone.
func TestNewestLinkOwnsTheCell(t *testing.T) {
	session, typed := linkedAgent(t)
	var got []string
	attach := func(name string) func() {
		detach, err := session.Attach("claude", func(delivery Delivery) error {
			got = append(got, name+":"+delivery.Text)
			return session.Delivered("claude", delivery.ID, true)
		})
		if err != nil {
			t.Fatalf("attach %s: %v", name, err)
		}
		return detach
	}
	detachOld := attach("old")
	detachNew := attach("new")

	for _, text := range []string{"one", "two"} {
		if err := session.Prompt(text); err != nil {
			t.Fatalf("prompt: %v", err)
		}
		if text == "one" {
			detachNew()
		}
	}
	if strings.Join(got, " ") != "new:one old:two" {
		t.Fatalf("the newest connection should own the cell, then the one before it: %v", got)
	}
	if err := session.Signal("claude", Replied, 1); err != nil || session.State() != Replied {
		t.Fatalf("with a connection left the mod should still be heard: %q, %v", session.State(), err)
	}

	detachOld()
	detachOld() // dropping twice takes nothing else down
	if err := session.Prompt("three"); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if len(got) != 2 || !strings.Contains(typed(), "three") {
		t.Fatalf("with no connection left the prompt is typed: delivered %v, typed %q", got, typed())
	}
}

// TestLinkAttachingDoesNotFreezeAScreenQuestion is the folder-trust dialog:
// Claude Code asks before it loads any mod, the cell reads the question off
// the screen, the user accepts — and only then does the mod attach. It knows
// nothing about that question and says nothing until the first turn, so the
// screen has to keep being read until it speaks.
func TestLinkAttachingDoesNotFreezeAScreenQuestion(t *testing.T) {
	turn := NewTurn(claudeMarkers)
	if state := turn.Observe("Do you trust the files in this folder?\n❯ 1. Yes, I trust this folder"); state != Approve {
		t.Fatalf("the trust dialog is a question, got %q", state)
	}

	turn.Link(true)
	if state := turn.Observe("Welcome back\n❯ \n? for shortcuts"); state != Working {
		t.Fatalf("the dialog was accepted before the mod attached, the cell still says %q", state)
	}

	// From its first word on, the mod is the one speaking.
	turn.Signal(Replied, 1)
	if state := turn.Observe("Do you want to proceed?"); state != Replied {
		t.Fatalf("after the mod spoke the screen has no say, got %q", state)
	}
}

// TestReconnectingKeepsAQuestionStillOnScreen — a mod that drops and comes
// back while the agent waits for a yes can't erase the wait: the question is
// still up, and the cell keeps saying so until the mod says otherwise.
func TestReconnectingKeepsAQuestionStillOnScreen(t *testing.T) {
	turn := NewTurn(claudeMarkers)
	turn.Link(true)
	turn.Signal(Approve, 1)

	turn.Link(false)
	turn.Link(true)
	if state := turn.Observe("Do you want to proceed?\n❯ 1. Yes"); state != Approve {
		t.Fatalf("the question is still on screen, the cell says %q", state)
	}
}
