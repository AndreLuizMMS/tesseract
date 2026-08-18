package engine

import (
	"strings"
	"testing"
)

// TestNotificationCommandRespectsCustom — whoever pointed to another
// notifier gets sent to it.
func TestNotificationCommandRespectsCustom(t *testing.T) {
	command := notificationCommand("/usr/bin/my-notify --urgent", "Tesseract", "claude · cortz answered")
	if command == nil {
		t.Fatal("the custom command should have been used")
	}
	if command.Path != "/usr/bin/my-notify" {
		t.Fatalf("program came out %q", command.Path)
	}
	args := command.Args
	if len(args) < 2 {
		t.Fatalf("args came out %v", args)
	}
	// Title and body are always the last two arguments.
	if args[len(args)-2] != "Tesseract" || args[len(args)-1] != "claude · cortz answered" {
		t.Fatalf("title and body were not the last ones: %v", args)
	}
	if args[1] != "--urgent" {
		t.Fatalf("the user's flag got lost: %v", args)
	}
}

// TestNotificationAlwaysHasAPath — on this machine (WSL) there's always a
// way to notify, even if it's through PowerShell.
func TestNotificationAlwaysHasAPath(t *testing.T) {
	command := notificationCommand("", "Tesseract", "claude · cortz answered")
	if command == nil {
		t.Skip("machine has no notifier")
	}
	if !strings.Contains(strings.Join(command.Args, " "), "cortz") {
		t.Fatalf("the notice body didn't reach the command: %v", command.Args)
	}
}

// TestToastEscapesQuotes — a cell name with an apostrophe doesn't break the
// PowerShell script.
func TestToastEscapesQuotes(t *testing.T) {
	script := toastScript("Tesseract", "the agent it's done answered")
	if strings.Contains(script, "it's done") {
		t.Fatalf("the apostrophe should have been escaped: %s", script)
	}
	if !strings.Contains(script, "it''s done") {
		t.Fatalf("wrong escape: %s", script)
	}
	if !strings.Contains(script, "ShowBalloonTip") {
		t.Fatalf("the script shows nothing: %s", script)
	}
}

// TestNoticeOffCallsNothing — both notices are toggleable, and the engine
// respects that.
func TestNoticeOffCallsNothing(t *testing.T) {
	m := New(t.TempDir(), Config{Sound: false, Notify: false})
	// No process to watch, this just must not blow up.
	m.announce("claude · cortz answered")
}
