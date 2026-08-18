package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/protocol"
)

func prepareFolders(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"cortz-web", "cortz-api", "doxar", ".hidden"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("preparing: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "cortz-annotations.md"), []byte("#"), 0o644); err != nil {
		t.Fatalf("preparing: %v", err)
	}
	return root
}

func TestCompleteOneCandidateGoesAllTheWay(t *testing.T) {
	root := prepareFolders(t)
	got := Complete(protocol.Complete{Path: filepath.Join(root, "dox"), DirsOnly: true})
	if got.Count != 1 {
		t.Fatalf("expected 1 candidate, got %d", got.Count)
	}
	if got.Path != filepath.Join(root, "doxar")+"/" {
		t.Fatalf("completed path came as %q", got.Path)
	}
}

func TestCompleteSeveralStopAtCommonPrefix(t *testing.T) {
	root := prepareFolders(t)
	got := Complete(protocol.Complete{Path: filepath.Join(root, "cor"), DirsOnly: true})
	if got.Count != 2 {
		t.Fatalf("expected 2 matching folders, got %d", got.Count)
	}
	if got.Path != filepath.Join(root, "cortz-") {
		t.Fatalf("should have stopped at the common prefix, got %q", got.Path)
	}
}

func TestCompleteDirectoryOnlyIgnoresFile(t *testing.T) {
	root := prepareFolders(t)
	withFile := Complete(protocol.Complete{Path: filepath.Join(root, "cortz-a")})
	if withFile.Count != 2 {
		t.Fatalf("without a filter, the file and the folder both match: got %d", withFile.Count)
	}
	onlyFolder := Complete(protocol.Complete{Path: filepath.Join(root, "cortz-a"), DirsOnly: true})
	if onlyFolder.Count != 1 || !strings.HasSuffix(onlyFolder.Path, "cortz-api/") {
		t.Fatalf("with the filter only the folder matches: %#v", onlyFolder)
	}
}

func TestCompleteNoCandidateReturnsWhatWasTyped(t *testing.T) {
	root := prepareFolders(t)
	typed := filepath.Join(root, "zzz")
	got := Complete(protocol.Complete{Path: typed, DirsOnly: true})
	if got.Count != 0 || got.Path != typed {
		t.Fatalf("with no candidate the field stays as it is: %#v", got)
	}
}

func TestCompleteHiddenOnlyWhenAskedFor(t *testing.T) {
	root := prepareFolders(t)
	all := Complete(protocol.Complete{Path: root + "/", DirsOnly: true})
	if all.Count != 3 {
		t.Fatalf("hidden folder should not show up on its own: got %d", all.Count)
	}
	asked := Complete(protocol.Complete{Path: filepath.Join(root, "."), DirsOnly: true})
	if asked.Count != 1 {
		t.Fatalf("asking for the dot, the hidden one matches: got %d", asked.Count)
	}
}
