package engine

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// forgetQuota discards the number cached between frames. Without this a test
// would read what the previous one left in memory, instead of the file it
// just wrote.
func forgetQuota(t *testing.T) {
	t.Helper()
	cachedQuota.Lock()
	cachedQuota.value, cachedQuota.read = nil, time.Time{}
	cachedQuota.Unlock()
}

// prepareQuota writes the file the user's statusline leaves, in a fake home
// dir.
func prepareQuota(t *testing.T, name string, percentage int, resetsAt time.Time, age time.Duration) {
	t.Helper()
	forgetQuota(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	path := filepath.Join(home, ".claude", name)
	body := `{"used_percentage":` + strconv.Itoa(percentage) + `,"resets_at":` + strconv.FormatInt(resetsAt.Unix(), 10) + `}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if age > 0 {
		old := time.Now().Add(-age)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatalf("aging: %v", err)
		}
	}
}

// TestQuotaReadsTheStatuslineFile — the number shows up with the time until
// the window resets.
func TestQuotaReadsTheStatuslineFile(t *testing.T) {
	prepareQuota(t, "tesseract-quota.json", 59, time.Now().Add(2*time.Hour+47*time.Minute), 0)

	quota := ReadQuota()
	if quota == nil {
		t.Fatal("the badge should show up")
	}
	if quota.Percent != 59 {
		t.Fatalf("percentage came out %d", quota.Percent)
	}
	if quota.Rollover != "2:46" && quota.Rollover != "2:47" {
		t.Fatalf("time until reset came out %q", quota.Rollover)
	}
}

// TestQuotaDisappearsWhenTheFileIsStale — Claude Code only delivers this
// number while a session is drawing the statusline, so a stale file is
// worthless.
func TestQuotaDisappearsWhenTheFileIsStale(t *testing.T) {
	prepareQuota(t, "tesseract-quota.json", 42, time.Now().Add(time.Hour), quotaStaleAfter+time.Minute)
	if quota := ReadQuota(); quota != nil {
		t.Fatalf("stale badge should disappear, came out %#v", quota)
	}
}

// TestQuotaWithoutFileDoesNotBreak.
func TestQuotaWithoutFileDoesNotBreak(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if quota := ReadQuota(); quota != nil {
		t.Fatalf("without a file there's no badge, came out %#v", quota)
	}
}

// TestQuotaAcceptsTheOldFile — whoever already had the fork's statusline
// keeps seeing the badge.
func TestQuotaAcceptsTheOldFile(t *testing.T) {
	prepareQuota(t, "squad-quota.json", 20, time.Now().Add(30*time.Minute), 0)
	quota := ReadQuota()
	if quota == nil || quota.Percent != 20 {
		t.Fatalf("the old file should have worked, came out %#v", quota)
	}
}

// TestQuotaAbove80ChangesRange — the range is what the bar uses to change
// color.
func TestQuotaAbove80ChangesRange(t *testing.T) {
	prepareQuota(t, "tesseract-quota.json", 87, time.Now().Add(time.Hour), 0)
	quota := ReadQuota()
	if quota == nil {
		t.Fatal("the badge should show up")
	}
	if quota.Percent < 80 {
		t.Fatalf("percentage came out %d", quota.Percent)
	}
}

// TestQuotaNeverExceedsHundred — a weird number in the file doesn't become a
// weird badge.
func TestQuotaNeverExceedsHundred(t *testing.T) {
	prepareQuota(t, "tesseract-quota.json", 250, time.Now().Add(time.Hour), 0)
	if quota := ReadQuota(); quota == nil || quota.Percent != 100 {
		t.Fatalf("percentage should cap at 100, came out %#v", quota)
	}
}

// TestQuotaWithUnreadableFileDoesNotBreak.
func TestQuotaWithUnreadableFileDoesNotBreak(t *testing.T) {
	forgetQuota(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "tesseract-quota.json"), []byte("this is not json"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if quota := ReadQuota(); quota != nil {
		t.Fatalf("unreadable file doesn't become a badge: %#v", quota)
	}
}
