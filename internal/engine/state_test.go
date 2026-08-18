package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSaveIsAtomic checks that writing leaves no half-written trace and that
// the final file is always readable.
func TestSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state := DesiredState{
		Screen: "grid",
		Projects: []SavedProject{{
			ID: "p1", Path: "/dev/cortz",
			Cells: []SavedCell{{ID: "c1", Kind: "bash", Name: "tests"}},
		}},
	}
	for range 5 {
		if err := Save(path, state); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read directory: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != "state.json" {
			t.Fatalf("a temp file was left behind after saving: %s", entry.Name())
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var check DesiredState
	if err := json.Unmarshal(content, &check); err != nil {
		t.Fatalf("saved state came out unreadable: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Projects) != 1 || loaded.Projects[0].Cells[0].Name != "tests" || loaded.Screen != "grid" {
		t.Fatalf("state came back different: %#v", loaded)
	}
}

// TestLoadUnreadableFilePreservesOriginal covers the worst case: the file
// turned to junk. Nothing is lost silently.
func TestLoadUnreadableFilePreservesOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	junk := []byte("{this isn't json")
	if err := os.WriteFile(path, junk, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	state, err := Load(path)
	if err == nil {
		t.Fatal("loading a corrupted file had to return an error")
	}
	if len(state.Projects) != 0 {
		t.Fatalf("nothing should have been loaded: %#v", state)
	}

	preserved, readErr := os.ReadFile(path + ".corrupted")
	if readErr != nil {
		t.Fatalf("the original was not preserved: %v", readErr)
	}
	if string(preserved) != string(junk) {
		t.Fatalf("preservation changed the content: %q", preserved)
	}
	if !strings.Contains(err.Error(), ".corrupted") {
		t.Fatalf("the error needs to say where the original ended up: %v", err)
	}
}

// TestLoadPartialUsesWhatItCan is the most common crash: one broken project
// among several good ones.
func TestLoadPartialUsesWhatItCan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	mixed := `{"screen":"list","projects":[
		{"id":"p1","path":"/dev/good","cells":[{"id":"c1","kind":"bash","name":"tests"}]},
		{"id":"p2","path":42},
		{"id":"p3","path":"/dev/other","cells":[]}
	]}`
	if err := os.WriteFile(path, []byte(mixed), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	state, err := Load(path)
	if err == nil {
		t.Fatal("discarding a project had to return an error")
	}
	if len(state.Projects) != 2 {
		t.Fatalf("the good projects should have survived, got %#v", state.Projects)
	}
	if state.Screen != "list" {
		t.Fatalf("the chosen screen got lost: %q", state.Screen)
	}
	if _, err := os.Stat(path + ".corrupted"); err != nil {
		t.Fatalf("the original was not preserved: %v", err)
	}
}

// TestLoadNoFile is the very first run: an empty grid, no error.
func TestLoadNoFile(t *testing.T) {
	state, err := Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("the first run cannot fail: %v", err)
	}
	if len(state.Projects) != 0 {
		t.Fatalf("the grid should start empty: %#v", state)
	}
}

// TestPreserveDoesNotOverwrite guarantees that two crashes in a row don't
// erase the first piece of evidence.
func TestPreserveDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	for i := range 2 {
		if err := os.WriteFile(path, []byte("junk"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("round %d: expected an error", i)
		}
	}
	if _, err := os.Stat(path + ".corrupted"); err != nil {
		t.Fatalf("first preservation is gone: %v", err)
	}
	if _, err := os.Stat(path + ".corrupted-1"); err != nil {
		t.Fatalf("second preservation was not created: %v", err)
	}
}
