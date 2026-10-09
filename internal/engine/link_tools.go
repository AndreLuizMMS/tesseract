package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/protocol"
)

// The tool mods (tesseract-md, tesseract-ide) draw nothing of their own: they
// tell the engine what the agent just wrote, and the panels that act on it —
// the markdown reader, the IDE picker — are the mosaic's.

// writtenFiles is what a cell's mods said its agent wrote last.
type writtenFiles struct {
	// doc is a markdown nobody opened yet; reading it clears it.
	doc string
	// file and line are the last edit, whatever the file.
	file string
	line int
}

// edited is the last edit the way the picker shows it: "path:line".
func (w writtenFiles) edited(root string) string {
	if w.file == "" {
		return ""
	}
	return inProject(root, w.file) + ":" + strconv.Itoa(w.line)
}

// inProject shortens a path to what it is inside the project. A file outside
// it keeps its whole path.
func inProject(root, path string) string {
	if relative, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(relative, "..") {
		return relative
	}
	return path
}

// reportFile keeps a file a mod said the agent wrote. The path came from
// outside the engine and ends up on a command line and in the reader: it has
// to be a real file, named whole.
func (e *Engine) reportFile(c *liveCell, report protocol.Report) error {
	path := filepath.Clean(report.File)
	if !filepath.IsAbs(path) || strings.ContainsFunc(path, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return fmt.Errorf("invalid file path")
	}
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a file", path)
	}

	e.mu.Lock()
	if report.Kind == "doc" {
		c.written.doc = path
	} else {
		c.written.file, c.written.line = path, max(report.Line, 1)
	}
	e.mu.Unlock()
	e.markDirty()
	return nil
}

// OpenDoc puts the markdown the cell's agent last wrote in front of the
// user: in the cell's own md tab when it has one, in a reading cell beside
// it when it doesn't.
func (e *Engine) OpenDoc(cellID string) error {
	c := e.findCell(cellID)
	if c == nil || c.proc == nil {
		return fmt.Errorf("cell %s does not exist", cellID)
	}
	e.mu.Lock()
	doc, root := c.written.doc, ""
	for _, p := range e.projects {
		for _, other := range p.cells {
			if other == c {
				root = p.path
			}
		}
	}
	e.mu.Unlock()
	if doc == "" {
		return fmt.Errorf("No new markdown from %s. tab switches to its md tab.", c.name)
	}

	if reader, ok := c.proc.(cell.WithDocs); ok {
		if err := reader.OpenDoc(doc); err != nil {
			return err
		}
	} else if _, err := e.Create(protocol.Create{Path: root, Type: "md", Target: doc}); err != nil {
		return err
	}

	e.mu.Lock()
	c.written.doc = ""
	if withTabs, ok := c.proc.(cell.WithTabs); ok {
		c.tab = withTabs.ActiveTab()
		e.save()
	}
	e.mu.Unlock()
	e.markDirty()
	return nil
}

// lastEdit is where the cell's agent last changed a file.
func (e *Engine) lastEdit(cellID string) (file string, line int, err error) {
	c := e.findCell(cellID)
	if c == nil {
		return "", 0, fmt.Errorf("cell %s does not exist", cellID)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if c.written.file == "" {
		return "", 0, fmt.Errorf("%s has not edited a file yet. ↵ opens the project.", c.name)
	}
	return c.written.file, c.written.line, nil
}
