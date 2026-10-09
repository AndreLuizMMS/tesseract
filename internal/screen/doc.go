package screen

import (
	"path/filepath"

	"github.com/andreluiz/tesseract/internal/keyboard"
)

// docHint is the one thing that's new about the focused cell: its agent
// wrote a markdown, and a key reads it. The key's own name comes from the
// keymap, and the file's name is what the user recognizes — the folders in
// front of it are in the reader's header. Nothing is said while there's no
// file.
func docHint(doc string) string {
	return hint(keyFor(keyboard.Browse, keyboard.OpenDoc) + " reads " + filepath.Base(doc))
}
