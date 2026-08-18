package engine

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/andreluiz/tesseract/internal/protocol"
)

// Complete resolves what the user has typed so far into a path and says how
// many candidates match — it's what the form's `tab` uses. A single candidate
// is completed in full; several stop at the common prefix.
func Complete(request protocol.Complete) protocol.Completed {
	raw := expandHome(request.Path)
	dir, start := filepath.Split(raw)
	if dir == "" {
		dir = "."
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return protocol.Completed{Path: request.Path}
	}

	var candidates []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, start) {
			continue
		}
		if start == "" && strings.HasPrefix(name, ".") {
			continue // a hidden folder only shows up when the user asks for it
		}
		if request.DirsOnly && !isDir(filepath.Join(dir, name)) {
			continue
		}
		candidates = append(candidates, name)
	}
	if len(candidates) == 0 {
		return protocol.Completed{Path: request.Path}
	}

	chosen := commonPrefix(candidates)
	complete := filepath.Join(dir, chosen)
	if len(candidates) == 1 && isDir(complete) {
		complete += string(os.PathSeparator)
	}
	return protocol.Completed{Path: complete, Count: len(candidates)}
}

// commonPrefix is as far as it can complete without choosing on its own.
func commonPrefix(names []string) string {
	common := names[0]
	for _, name := range names[1:] {
		for !strings.HasPrefix(name, common) {
			common = common[:len(common)-1]
			if common == "" {
				return ""
			}
		}
	}
	return common
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// expandHome swaps ~ for the account's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}
