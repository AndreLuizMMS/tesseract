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

var cachedQuota struct {
	sync.Mutex
	value *protocol.Quota
	read  time.Time
}

// ReadQuota returns the 5-hour window's usage, or nil when the data doesn't
// exist or is stale. Without the file, everything still works — the badge
// just doesn't show up.
func ReadQuota() *protocol.Quota {
	now := time.Now()
	cachedQuota.Lock()
	defer cachedQuota.Unlock()
	if now.Sub(cachedQuota.read) < quotaCacheTTL {
		return cachedQuota.value
	}
	cachedQuota.value, cachedQuota.read = readQuotaFromDisk(), now
	return cachedQuota.value
}

func readQuotaFromDisk() *protocol.Quota {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	for _, name := range quotaFiles {
		if quota := readQuotaFile(filepath.Join(home, ".claude", name)); quota != nil {
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
