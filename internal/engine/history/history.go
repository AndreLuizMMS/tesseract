// Package history keeps everything a cell produced, in its own file,
// with a size cap and discarding of the start. It's what backs scrolling and
// search after a WSL crash.
package history

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// DefaultCap is the maximum size of a cell's history. Past that, the
// start is discarded and the end is kept.
const DefaultCap int64 = 5 << 20

// keepFraction is the slice of the cap that stays in the file after a
// discard. Discarding only the excess would make the cut happen on every
// write.
const keepFraction = 3.0 / 4.0

// History is a cell's file.
type History struct {
	mu   sync.Mutex
	path string
	cap  int64
	file *os.File
	size int64
}

// Match is a history line that matched the search.
type Match struct {
	Line int    // 1-based, over the file's current content
	Text string // already stripped of terminal codes
}

// Open prepares the cell's history, continuing whatever file already exists.
func Open(path string, cap int64) (*History, error) {
	if cap <= 0 {
		cap = DefaultCap
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	return &History{path: path, cap: cap, file: file, size: info.Size()}, nil
}

// Write records what the cell produced and discards the start when the cap
// is exceeded.
func (h *History) Write(p []byte) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	n, err := h.file.Write(p)
	h.size += int64(n)
	if err != nil {
		return n, err
	}
	if h.size > h.cap {
		if err := h.discardStart(); err != nil {
			return n, err
		}
	}
	return n, nil
}

// discardStart rewrites the file keeping only the end. Called with the lock
// already held.
func (h *History) discardStart() error {
	keep := int64(float64(h.cap) * keepFraction)
	source, err := os.Open(h.path)
	if err != nil {
		return err
	}
	defer source.Close()
	if _, err := source.Seek(h.size-keep, io.SeekStart); err != nil {
		return err
	}

	tmp := h.path + ".cutting"
	dest, err := os.Create(tmp)
	if err != nil {
		return err
	}
	copied, err := io.Copy(dest, source)
	if cerr := dest.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, h.path); err != nil {
		os.Remove(tmp)
		return err
	}

	newFile, err := os.OpenFile(h.path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	h.file.Close()
	h.file = newFile
	h.size = copied
	return nil
}

// Tail returns the last n bytes written, which is what the engine replays on
// the cell's screen when it comes back after a crash.
func (h *History) Tail(n int64) ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if n > h.size {
		n = h.size
	}
	if n <= 0 {
		return nil, nil
	}
	buf := make([]byte, n)
	if _, err := h.file.ReadAt(buf, h.size-n); err != nil && err != io.EOF {
		return nil, err
	}
	return buf, nil
}

// Search scans the history and returns the lines containing the term, with
// the line number. The comparison is case-insensitive.
func (h *History) Search(term string) ([]Match, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if term == "" {
		return nil, nil
	}
	file, err := os.Open(h.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	target := strings.ToLower(term)
	var matches []Match
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		text := StripCodes(scanner.Text())
		if strings.Contains(strings.ToLower(text), target) {
			matches = append(matches, Match{Line: lineNum, Text: text})
		}
	}
	if err := scanner.Err(); err != nil {
		return matches, fmt.Errorf("reading %s: %w", h.path, err)
	}
	return matches, nil
}

// Lines counts how many lines the history currently has, which is the
// universe search uses to number what it finds.
func (h *History) Lines() (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	file, err := os.Open(h.path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	total := 0
	for scanner.Scan() {
		total++
	}
	return total, scanner.Err()
}

// Size is how much the history currently occupies.
func (h *History) Size() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.size
}

// Close releases the file.
func (h *History) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.file.Close()
}

// StripCodes removes terminal control codes from the text, leaving only
// what a person would read on the screen.
func StripCodes(raw string) string {
	var clean strings.Builder
	clean.Grow(len(raw))
	for i := 0; i < len(raw); {
		c := raw[i]
		switch {
		case c == 0x1b && i+1 < len(raw) && raw[i+1] == '[':
			// CSI sequence: ends at the first byte from 0x40 to 0x7e.
			i += 2
			for i < len(raw) && (raw[i] < 0x40 || raw[i] > 0x7e) {
				i++
			}
			i++
		case c == 0x1b && i+1 < len(raw) && (raw[i+1] == ']' || raw[i+1] == 'P' || raw[i+1] == '^' || raw[i+1] == '_'):
			// String sequence: ends at BEL or ESC \.
			i += 2
			for i < len(raw) {
				if raw[i] == 0x07 {
					i++
					break
				}
				if raw[i] == 0x1b && i+1 < len(raw) && raw[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		case c == 0x1b:
			i += 2
		case c == '\r':
			i++
		case c < 0x20 && c != '\t':
			i++
		default:
			clean.WriteByte(c)
			i++
		}
	}
	return clean.String()
}
