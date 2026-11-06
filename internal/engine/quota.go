package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/andreluiz/tesseract/internal/protocol"
)

// quotaStaleAfter caps the age of the usage file. Claude Code only delivers
// this number while a session is drawing the statusline, so an old file means
// nothing has run for a while — and the number no longer means anything.
const quotaStaleAfter = 15 * time.Minute

// quotaFiles are the places where the user's statusline leaves the 5-hour
// window's usage, in order of preference.
var quotaFiles = []string{"tesseract-quota.json", "squad-quota.json"}

// quotaCacheTTL is how long the number that was read stays valid. The
// engine's snapshot goes out up to 25 times per second, and usage changes
// minute to minute: hitting disk every frame would be wasted work.
const quotaCacheTTL = 2 * time.Second

// cachedQuota is the statusline file's number, per account folder.
var cachedQuota struct {
	sync.Mutex
	by map[string]fileQuota
}

type fileQuota struct {
	value *protocol.Quota
	read  time.Time
}

// reportedQuota is the usage the cells' mods measured from inside Claude
// Code, per account folder: each report counts for the account of the cell
// that sent it, never for another. It's the first source: the statusline
// file only speaks when no mod has.
var reportedQuota struct {
	sync.Mutex
	by map[string]quotaReport
}

type quotaReport struct {
	percent  int
	weekly   int
	rollover time.Time
	at       time.Time
}

// reportedQuotaAt is the mod's number for the account, while it's fresh.
// The time left is worked out on every reading, because the clock runs
// between two reports.
func reportedQuotaAt(account string, now time.Time) *protocol.Quota {
	reportedQuota.Lock()
	defer reportedQuota.Unlock()
	report := reportedQuota.by[account]
	if report.at.IsZero() || now.Sub(report.at) > quotaStaleAfter {
		return nil
	}
	quota := &protocol.Quota{Percent: report.percent, Weekly: report.weekly}
	if !report.rollover.IsZero() {
		quota.Rollover = timeRemaining(report.rollover.Sub(now))
	}
	return quota
}

// ReadQuota returns the account's 5-hour window usage, or nil when the data
// doesn't exist or is stale. A mod's report comes first and the statusline
// file in the account's own folder is what's left without one; with
// neither, everything still works — the badge just doesn't show up.
func ReadQuota(account string) *protocol.Quota {
	now := time.Now()
	if quota := reportedQuotaAt(account, now); quota != nil {
		return quota
	}
	cachedQuota.Lock()
	defer cachedQuota.Unlock()
	if cached := cachedQuota.by[account]; now.Sub(cached.read) < quotaCacheTTL {
		return cached.value
	}
	if cachedQuota.by == nil {
		cachedQuota.by = map[string]fileQuota{}
	}
	value := readQuotaFromDisk(account)
	cachedQuota.by[account] = fileQuota{value: value, read: now}
	return value
}

func readQuotaFromDisk(account string) *protocol.Quota {
	for _, name := range quotaFiles {
		if quota := readQuotaFile(filepath.Join(account, name)); quota != nil {
			return quota
		}
	}
	return nil
}

func readQuotaFile(path string) *protocol.Quota {
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) > quotaStaleAfter {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var raw struct {
		UsedPercentage float64 `json:"used_percentage"`
		ResetsAt       int64   `json:"resets_at"`
	}
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil
	}

	quota := &protocol.Quota{Percent: min(max(int(raw.UsedPercentage+0.5), 0), 100)}
	if raw.ResetsAt > 0 {
		quota.Rollover = timeRemaining(time.Until(time.Unix(raw.ResetsAt, 0)))
	}
	return quota
}

// timeRemaining writes the time until the window resets as h:mm.
func timeRemaining(remaining time.Duration) string {
	if remaining <= 0 {
		return "0:00"
	}
	hours := int(remaining.Hours())
	minutes := int(remaining.Minutes()) % 60
	text := strconv.Itoa(hours) + ":"
	if minutes < 10 {
		text += "0"
	}
	return text + strconv.Itoa(minutes)
}
