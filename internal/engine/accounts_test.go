package engine

import (
	"testing"
	"time"
)

// TestPrimaryAccountComesFirstWhenFree is the rule the user asked for: every
// cell runs on the primary unless it's at its limit, goes back to it the
// moment the limit resets, and with every account out waits on the one that
// comes back first.
func TestPrimaryAccountComesFirstWhenFree(t *testing.T) {
	t.Setenv("HOME", "/home/someone")
	a := newAccounts([]string{"~/.claude", "/srv/claude-2/", " "})
	primary, second := "/home/someone/.claude", "/srv/claude-2"
	if len(a.folders) != 2 || a.current() != primary {
		t.Fatalf("folders %v, current %q: want the primary first, ~ expanded", a.folders, a.current())
	}

	a.exhaust(primary, time.Now().Add(time.Hour))
	if folder, free := a.pick(); folder != second || !free {
		t.Fatalf("primary out: got %q free=%v, want the second account", folder, free)
	}

	a.exhaust(second, time.Now().Add(30*time.Minute))
	if folder, free := a.pick(); folder != second || free {
		t.Fatalf("both out: got %q free=%v, want the one back first, not free", folder, free)
	}

	a.exhaust(primary, time.Now().Add(-time.Second))
	if a.current() != primary {
		t.Fatal("a primary whose limit reset is the one cells run on again")
	}
	if newAccounts(nil).current() != "" {
		t.Fatal("no accounts configured means no choice: the cell keeps Claude Code's own folder")
	}
}
