package cell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/engine/history"
)

// testSession brings up a session with fake agents and a history per tab.
func testSession(t *testing.T) (*Session, string) {
	t.Helper()
	dir := t.TempDir()
	fake := fakeAgent(t, dir)

	session := &Session{}
	cfg := Config{
		ID: "c1", Directory: dir, Name: "work", Columns: 60, Lines: 12,
		Profiles: map[string]Profile{
			"claude": {Program: fake},
			"cursor": {Program: fake},
		},
		OpenHistory: func(suffix string) (*history.History, error) {
			return history.Open(filepath.Join(dir, "hist-"+suffix+".log"), history.DefaultCap)
		},
	}
	if err := session.Spawn(cfg); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	t.Cleanup(func() { session.Kill() })
	return session, dir
}

// TestSessionSpawnsWithTabsWithoutAskingAnything — creating a session doesn't
// force a choice between claude, cursor and shell: it already comes with all
// three.
func TestSessionSpawnsWithTabsWithoutAskingAnything(t *testing.T) {
	session, _ := testSession(t)

	tabs := session.Tabs()
	if len(tabs) < 3 {
		t.Fatalf("the session should spawn with the agents' tabs, got %v", tabs)
	}
	if session.ActiveTab() != tabs[0] {
		t.Fatalf("the first tab should be active, it's %q", session.ActiveTab())
	}
	if session.State() != Working {
		t.Fatalf("the active tab should be up, it's %q", session.State())
	}
}

// TestSessionOnlyBringsUpTheTabInUse — three agents per session would cost a
// lot; the tabs come up when someone switches to them.
func TestSessionOnlyBringsUpTheTabInUse(t *testing.T) {
	session, dir := testSession(t)

	if len(session.open) != 1 {
		t.Fatalf("only the active tab should be up, %d came up", len(session.open))
	}
	if _, err := os.Stat(filepath.Join(dir, "hist-cursor.log")); err == nil {
		t.Fatal("the tab nobody opened shouldn't have a history yet")
	}

	if err := session.SwitchTab(1); err != nil {
		t.Fatalf("switch tab: %v", err)
	}
	if session.ActiveTab() != "cursor" {
		t.Fatalf("the active tab should be cursor, it's %q", session.ActiveTab())
	}
	if len(session.open) != 2 {
		t.Fatalf("the new tab should have come up, we have %d", len(session.open))
	}
	if _, err := os.Stat(filepath.Join(dir, "hist-cursor.log")); err != nil {
		t.Fatalf("each tab has its own history: %v", err)
	}
}

// TestSessionLoopsAroundTheTabs — the key wraps around, in both directions.
func TestSessionLoopsAroundTheTabs(t *testing.T) {
	session, _ := testSession(t)
	tabs := session.Tabs()

	for i := 1; i <= len(tabs); i++ {
		if err := session.SwitchTab(1); err != nil {
			t.Fatalf("switch tab: %v", err)
		}
		expected := tabs[i%len(tabs)]
		if session.ActiveTab() != expected {
			t.Fatalf("step %d: expected %q, got %q", i, expected, session.ActiveTab())
		}
	}
	if err := session.SwitchTab(-1); err != nil {
		t.Fatalf("switch back: %v", err)
	}
	if session.ActiveTab() != tabs[len(tabs)-1] {
		t.Fatalf("going back should land on the last tab, it's %q", session.ActiveTab())
	}
}

// TestSessionShowsTheActiveTab — what the screen draws is the current tab's
// content.
func TestSessionShowsTheActiveTab(t *testing.T) {
	session, _ := testSession(t)

	// Go to the shell tab, the one a test can write to.
	for session.ActiveTab() != "bash" {
		if err := session.SwitchTab(1); err != nil {
			t.Fatalf("switch tab: %v", err)
		}
	}
	pasteAndEnter(t, session, "echo dentro-da-aba")
	if !waitFor(t, 3*time.Second, func() bool {
		return strings.Contains(strings.Join(session.Draw().Lines, "\n"), "dentro-da-aba")
	}) {
		t.Fatalf("the active tab's output didn't show up:\n%s", strings.Join(session.Draw().Lines, "\n"))
	}

	// Search looks at the history of the tab showing right now.
	record := session.ActiveHistory()
	if record == nil {
		t.Fatal("the active tab should have a history")
	}
	if !waitFor(t, 2*time.Second, func() bool {
		found, _ := record.Search("dentro-da-aba")
		return len(found) > 0
	}) {
		t.Fatal("the active tab's history didn't record what it wrote")
	}
}

// TestSessionKeepsEachTabsConversation — after a crash, each tab reattaches
// its own.
func TestSessionKeepsEachTabsConversation(t *testing.T) {
	session, _ := testSession(t)
	if !waitFor(t, 3*time.Second, func() bool { return session.Conversations()["claude"] != "" }) {
		t.Fatalf("the claude tab's conversation should have an identity: %#v", session.Conversations())
	}
	if session.Conversations()["bash"] != "" {
		t.Fatal("a shell has no conversation")
	}
}

// TestSessionKillsAllTabs — killing the session takes every tab with it.
func TestSessionKillsAllTabs(t *testing.T) {
	session, _ := testSession(t)
	if err := session.SwitchTab(1); err != nil {
		t.Fatalf("switch tab: %v", err)
	}
	open := len(session.open)
	if open < 2 {
		t.Fatalf("two tabs should be open, %d are", open)
	}
	if err := session.Kill(); err != nil {
		t.Fatalf("kill: %v", err)
	}
	for tab, item := range session.open {
		if !waitFor(t, 3*time.Second, func() bool { return item.State() == Stopped }) {
			t.Fatalf("the %s tab stayed up: %q", tab, item.State())
		}
	}
}

// TestSessionSpawnsOnTheSavedTab — reconstructing goes back to the tab that
// was showing.
func TestSessionSpawnsOnTheSavedTab(t *testing.T) {
	dir := t.TempDir()
	fake := fakeAgent(t, dir)
	session := &Session{}
	if err := session.Spawn(Config{
		ID: "c1", Directory: dir, Columns: 60, Lines: 12, Tab: "bash",
		Profiles: map[string]Profile{"claude": {Program: fake}, "cursor": {Program: fake}},
	}); err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer session.Kill()

	if session.ActiveTab() != "bash" {
		t.Fatalf("should have spawned on the saved tab, it's on %q", session.ActiveTab())
	}
}

// TestSessionNotifiesTheTabThatCameBackIntoView — the markdown tab looks for
// a new file when it comes back into view, not once a minute in the dark.
func TestSessionNotifiesTheTabThatCameBackIntoView(t *testing.T) {
	session, dir := testSession(t)

	// Go to the markdown tab.
	for session.ActiveTab() != "md" {
		if err := session.SwitchTab(1); err != nil {
			t.Fatalf("switch tab: %v", err)
		}
	}
	if !waitFor(t, 2*time.Second, func() bool {
		return strings.Contains(strings.Join(session.Draw().Lines, "\n"), "search:")
	}) {
		t.Fatalf("the markdown tab should show the search box:\n%s", strings.Join(session.Draw().Lines, "\n"))
	}

	// A new file appears on disk while the tab is out of focus.
	if err := session.SwitchTab(1); err != nil {
		t.Fatalf("switch tab: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "NOVO-DOCUMENTO.md"), []byte("# novo\n"), 0o644); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	for session.ActiveTab() != "md" {
		if err := session.SwitchTab(1); err != nil {
			t.Fatalf("switch tab: %v", err)
		}
	}
	if !waitFor(t, 3*time.Second, func() bool {
		return strings.Contains(strings.Join(session.Draw().Lines, "\n"), "NOVO-DOCUMENTO.md")
	}) {
		t.Fatalf("the tab should have rechecked on coming back:\n%s", strings.Join(session.Draw().Lines, "\n"))
	}
}
