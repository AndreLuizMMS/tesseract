package cell

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/andreluiz/tesseract/internal/engine/history"
)

// prepareKind sets up what each type needs to be born in a test directory,
// without depending on a real agent or a running stack.
func prepareKind(t *testing.T, kind, dir string) Config {
	t.Helper()
	cfg := Config{ID: "c-" + kind, Directory: dir, Name: kind, Columns: 60, Lines: 12}

	switch kind {
	case "claude", "cursor", "session":
		// Fake agents, which accept any argument and stay up: the contract
		// belongs to the cell, not the agent.
		fake := fakeAgent(t, dir)
		cfg.Profiles = map[string]Profile{
			"claude": {Program: fake},
			"cursor": {Program: fake},
		}
	case "shell":
		// A fake `docker` that stays up: the contract is the cell's, not the
		// stack's, and there's no container to enter in a test.
		bin := filepath.Join(dir, "bin")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatalf("prepare bin: %v", err)
		}
		if err := os.WriteFile(filepath.Join(bin, "docker"), []byte("#!/bin/sh\nwhile true; do sleep 1; done\n"), 0o755); err != nil {
			t.Fatalf("prepare fake docker: %v", err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		fallthrough
	case "logs":
		compose := filepath.Join(dir, "docker-compose.yml")
		if err := os.WriteFile(compose, []byte("services:\n  web:\n    image: nginx\n"), 0o644); err != nil {
			t.Fatalf("prepare compose: %v", err)
		}
		cfg.Target = "web"
	case "md":
		file := filepath.Join(dir, "leia.md")
		if err := os.WriteFile(file, []byte("# Title\n\nfile text\n"), 0o644); err != nil {
			t.Fatalf("prepare markdown: %v", err)
		}
		cfg.Target = file
	}
	return cfg
}

// fakeAgent is a program that ignores its arguments and stays alive, so the
// agent types can be born without depending on the real agent.
func fakeAgent(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "fake-agent")
	body := "#!/bin/sh\nwhile true; do sleep 1; done\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("prepare agent: %v", err)
	}
	return path
}

// TestEveryKindMeetsTheContract walks the whole registry: a new type gets
// covered for free, and if it doesn't answer to spawn, draw, receive a
// keystroke and report states, this test breaks.
func TestEveryKindMeetsTheContract(t *testing.T) {
	kinds := Types()
	if len(kinds) < 5 {
		t.Fatalf("the registry should have the five V1 types, has %v", kinds)
	}

	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			cfg := prepareKind(t, kind, dir)

			record, err := history.Open(filepath.Join(dir, "hist.log"), history.DefaultCap)
			if err != nil {
				t.Fatalf("open history: %v", err)
			}
			defer record.Close()
			cfg.History = record

			item, err := New(kind)
			if err != nil {
				t.Fatalf("manufacture: %v", err)
			}

			// Spawns.
			if err := item.Spawn(cfg); err != nil {
				t.Fatalf("spawn: %v", err)
			}
			defer item.Kill()

			// Reports the states it has, and the current state is one of them.
			states := item.States()
			if len(states) == 0 {
				t.Fatal("the type declares no state")
			}
			time.Sleep(200 * time.Millisecond)
			current := item.State()
			declared := false
			for _, state := range states {
				if state == current {
					declared = true
				}
			}
			if !declared {
				t.Fatalf("the current state %q isn't among the declared ones %v", current, states)
			}

			// Draws.
			frame := item.Draw()
			if frame.Lines == nil && frame.CursorX == 0 && frame.CursorY == 0 && !frame.Live {
				t.Fatal("draw returned no frame at all")
			}

			// Receives a keystroke without exploding (read-only ones ignore it).
			if err := item.Key(Keystroke{Code: 'a', Text: "a"}); err != nil {
				t.Fatalf("key: %v", err)
			}

			// Accepts size and scroll.
			if err := item.Resize(80, 20); err != nil {
				t.Fatalf("resize: %v", err)
			}
			item.Scroll(3, false)
			item.Scroll(0, true)

			// Dies.
			if err := item.Kill(); err != nil {
				t.Fatalf("kill: %v", err)
			}
		})
	}
}

// TestDescriptorsAreConsistent — the form builds its fields from the
// descriptors, so they need to be complete.
func TestDescriptorsAreConsistent(t *testing.T) {
	for _, descriptor := range Descriptors() {
		if descriptor.Type == "" {
			t.Fatal("descriptor without a type")
		}
		if descriptor.TargetIsPath && descriptor.TargetLabel == "" {
			t.Errorf("%s: targets a path in a field that doesn't exist", descriptor.Type)
		}
		if _, exists := Describe(descriptor.Type); !exists {
			t.Errorf("%s: registered but has no descriptor", descriptor.Type)
		}
	}
	if descriptor, _ := Describe("md"); descriptor.AcceptsPrompt {
		t.Error("markdown doesn't take a prompt: the cell only reads")
	}
	if descriptor, _ := Describe("claude"); !descriptor.AcceptsPrompt || !descriptor.HasConversation {
		t.Error("claude accepts a prompt and has a conversation")
	}
	if descriptor, _ := Describe("logs"); descriptor.TargetLabel == "" {
		t.Error("the logs cell needs to ask which service")
	}
}

// TestAgentWithoutTranscriptDoesNotTryToResume is what keeps the cell from
// dying at the start after a crash: a conversation that never reached disk
// starts over with the same identity instead of being resumed.
func TestAgentWithoutTranscriptDoesNotTryToResume(t *testing.T) {
	if hasTranscript(t.TempDir(), t.TempDir(), "99b7c1fb-cb36-4485-b29c-324c994d4607") {
		t.Fatal("a conversation that never existed can't look resumable")
	}
	if hasTranscript(t.TempDir(), t.TempDir(), "") {
		t.Fatal("a conversation without an identity isn't resumable")
	}
	if cursorHasConversation("conversation-that-does-not-exist") {
		t.Fatal("a nonexistent Cursor conversation can't look resumable")
	}
}

// TestConversationFollowsTheNewestTranscript is what keeps the context when a
// conversation changes accounts: the account it lands on adopts the newest
// copy, and an older copy never overwrites a newer one.
func TestConversationFollowsTheNewestTranscript(t *testing.T) {
	const conversation = "99b7c1fb-cb36-4485-b29c-324c994d4607"
	primary, second, project := t.TempDir(), t.TempDir(), "/home/someone/app"
	write := func(account, text string, at time.Time) {
		folder := claudeFolder(account, project)
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(folder, conversation+".jsonl")
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	read := func(account string) string {
		content, _ := os.ReadFile(filepath.Join(claudeFolder(account, project), conversation+".jsonl"))
		return string(content)
	}
	accounts := []string{primary, second}

	write(primary, "turns on the primary", time.Now().Add(-time.Hour))
	adoptTranscript(accounts, second, project, conversation)
	if read(second) != "turns on the primary" || !hasTranscript(second, project, conversation) {
		t.Fatalf("second account got %q: it should resume the primary's conversation", read(second))
	}

	write(second, "turns on the primary + on the second", time.Now())
	adoptTranscript(accounts, primary, project, conversation)
	if read(primary) != "turns on the primary + on the second" {
		t.Fatalf("primary got %q: coming back must bring what was said on the second", read(primary))
	}

	write(primary, "older", time.Now().Add(-2*time.Hour))
	write(second, "newer", time.Now())
	adoptTranscript(accounts, second, project, conversation)
	if read(second) != "newer" {
		t.Fatal("an older copy overwrote a newer one")
	}
}

// TestSecondAccountTrustsWhatThePrimaryTrusts keeps a conversation moved to
// another account from coming back up stuck on the folder-trust question —
// and keeps a folder never trusted from being trusted behind the user's back.
func TestSecondAccountTrustsWhatThePrimaryTrusts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	native := `{"userID":"x","firstStartTime":12345678901234567890,"projects":{"/srv/app":{"hasTrustDialogAccepted":true}}}`
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(native), 0o600); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(home, ".claude-2")
	if err := os.MkdirAll(second, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, ".claude.json"), []byte(`{"oauthAccount":{"emailAddress":"b@x"},"big":98765432109876543210}`), 0o600); err != nil {
		t.Fatal(err)
	}

	carryTrust(second, "/srv/app/")
	carryTrust(second, "/srv/other")
	state, err := readState(filepath.Join(second, ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !trusted(state, "/srv/app") {
		t.Fatal("a folder the primary trusts must be trusted on the second account")
	}
	if trusted(state, "/srv/other") {
		t.Fatal("a folder the primary never trusted was trusted on the second account")
	}
	if state["oauthAccount"] == nil || fmt.Sprint(state["big"]) != "98765432109876543210" {
		t.Fatalf("the rest of the file changed: %v", state)
	}
}
