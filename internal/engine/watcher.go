package engine

import (
	"os"
	"path/filepath"
	"time"

	"github.com/andreluiz/tesseract/internal/cell"
	"github.com/andreluiz/tesseract/internal/docker"
)

const (
	// watchInterval is how often the engine looks at the cells' state to
	// decide whether anyone needs to be notified.
	watchInterval = 500 * time.Millisecond
	// ticksBetweenDiskChecks spaces out checking that the projects'
	// directories still exist.
	ticksBetweenDiskChecks = 20
	// ticksBetweenDockerChecks spaces out reading each project's stack
	// state, which is what the strip shows.
	ticksBetweenDockerChecks = 40
	// ticksBetweenRefresh is the slow heartbeat that redraws the grid even
	// without any change. It serves what tracks the clock and doesn't notify
	// anyone — the quota counter. Otherwise, a still grid produces no frame
	// at all.
	ticksBetweenRefresh = 10
)

// Watch tracks the cells and notifies when one starts asking for attention.
// It's the engine that notifies, not the screen: the user gets notified even
// with the screen closed.
func (e *Engine) Watch() {
	clock := time.NewTicker(watchInterval)
	defer clock.Stop()
	tick := 0
	for {
		select {
		case <-e.stop:
			return
		case <-clock.C:
			tick++
			changed := e.checkStates()
			if tick%ticksBetweenDiskChecks == 0 {
				changed = e.checkDirectories() || changed
			}
			if tick%ticksBetweenDockerChecks == 1 {
				changed = e.checkDocker() || changed
			}
			// A still grid sends no snapshot: whoever draws on the other
			// side has nothing to redraw, and the engine doesn't pay for
			// JSON for nothing.
			if changed || tick%ticksBetweenRefresh == 0 {
				e.markDirty()
			}
		}
	}
}

// stopWatcher shuts the watcher down along with the engine.
func (e *Engine) stopWatcher() {
	e.once.Do(func() { close(e.stop) })
}

// checkStates announces the changes worth a notice: who answered, who asked
// for approval, and who died.
func (e *Engine) checkStates() bool {
	type notice struct{ text string }
	var notices []notice
	var toAdoptName []string
	changed := false

	e.mu.Lock()
	for _, p := range e.projects {
		projectName := filepath.Base(p.path)
		for _, c := range p.cells {
			if c.proc == nil {
				continue
			}
			state := c.proc.State()
			if state == c.lastState {
				continue
			}
			previous := c.lastState
			c.lastState = state
			changed = true
			switch state {
			case cell.Replied:
				notices = append(notices, notice{c.name + " · " + projectName + " answered"})
				if c.pendingRename {
					toAdoptName = append(toAdoptName, c.id)
				}
			case cell.Approve:
				notices = append(notices, notice{c.name + " · " + projectName + " requested approval"})
			case cell.Crashed:
				if previous != cell.Stopped {
					notices = append(notices, notice{c.name + " · " + projectName + " crashed"})
				}
			}
		}
	}
	e.mu.Unlock()

	for _, n := range notices {
		e.announce(n.text)
	}
	// Outside the lock: adopting the name calls Rename, which locks the
	// mutex again.
	for _, id := range toAdoptName {
		_ = e.AdoptAgentName(id)
	}
	return changed
}

// checkDirectories marks as orphaned the cell whose project has disappeared
// from disk.
func (e *Engine) checkDirectories() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	changed := false
	for _, p := range e.projects {
		if info, err := os.Stat(p.path); err == nil && info.IsDir() {
			continue
		}
		for _, c := range p.cells {
			changed = changed || c.lastState != cell.Orphaned
			c.lastState = cell.Orphaned
		}
	}
	return changed
}

// checkDocker refreshes each project's stack summary, for projects that have
// a compose file. Docker never comes up on its own: this only reads.
func (e *Engine) checkDocker() bool {
	e.mu.Lock()
	type target struct {
		p       *project
		path    string
		compose string
	}
	var targets []target
	for _, p := range e.projects {
		if p.compose != "" {
			targets = append(targets, target{p: p, path: p.path, compose: p.compose})
		}
	}
	e.mu.Unlock()

	changed := false
	for _, a := range targets {
		services, err := docker.Services(a.path, a.compose)
		summary := ""
		if err == nil {
			summary = docker.Summary(services)
		}
		e.mu.Lock()
		changed = changed || a.p.docker != summary
		a.p.docker = summary
		e.mu.Unlock()
	}
	return changed
}
