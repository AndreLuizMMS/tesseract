package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestAccountGoesToTheEndOfTheConfig keeps the user's other keys, puts
// Claude Code's own folder first when there was no list, and never lists
// the same account twice.
func TestAccountGoesToTheEndOfTheConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	path := filepath.Join(home, ".config", "tesseract", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"editor": "code"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(home, ".claude-2")

	accounts, err := addToConfig(path, nil, second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := addToConfig(path, accounts, second); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(path)
	var written struct {
		Editor   string   `json:"editor"`
		Accounts []string `json:"claudeAccounts"`
	}
	if err := json.Unmarshal(content, &written); err != nil {
		t.Fatal(err)
	}
	if written.Editor != "code" {
		t.Fatalf("the user's own key was lost: %s", content)
	}
	if len(written.Accounts) != 2 || written.Accounts[0] != "~/.claude" || written.Accounts[1] != "~/.claude-2" {
		t.Fatalf("accounts %v: want the primary first and the new one once", written.Accounts)
	}
	if freeAccountName(accounts) != "3" {
		t.Fatal("the next account should be the first number not listed")
	}
}
