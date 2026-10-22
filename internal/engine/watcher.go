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
	// ticksBetweenRenameChecks spaces out reading the transcript of a cell
	// waiting for its /rename: the file can be tens of megabytes.
	ticksBetweenRenameChecks = 4
	// renameWait is how long that wait lasts. A /rename sent mid-turn only
	// runs when the turn ends.
	renameWait = 10 * time.Minute
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
			e.watchTick(tick)
		}
	}
}

// watchTick is one look at the grid. Most ticks only read the cells' state;
// the disk, Docker and the heartbeat each come around on their own count.
func (e *Engine) watchTick(tick int) {
	changed := e.checkStates()
	if tick%ticksBetweenRenameChecks == 0 {
		e.checkRenames()
	}
	if tick%ticksBetweenDiskChecks == 0 {
		changed = e.checkDirectories() || changed
	}
	if tick%ticksBetweenDockerChecks == 1 {
		changed = e.checkDocker() || changed
	}
	// A still grid sends no snapshot: whoever draws on the other side has
	// nothing to redraw, and the engine doesn't pay for JSON for nothing.
	if changed || tick%ticksBetweenRefresh == 0 {
		e.markDirty()
	}
}

// stopWatcher shuts the watcher down along with the engine.
func (e *Engine) stopWatcher() {
	e.once.Do(func() { close(e.stop) })
}

// checkStates announces the changes worth a notice: who responded, who is
// waiting on an approve, and who died. The words are the ones the cell's own
// marker uses — the notice and the screen can't name one state two ways.
func (e *Engine) checkStates() bool {
	type notice struct{ text string }
	var notices []notice
	changed := false

	e.mu.Lock()
	for _, p := range e.projects {
		projectName := filepath.Base(p.path)
		for _, c := range p.cells {
			if c.proc == nil {
				continue
			}
			state := c.proc.State()
			// A cell with tabs answers with the state of the tab in view.
			// Changing tabs changes the answer, and that's the user looking
			// at the cell, not the agent saying something new: a turn that
			// ended is announced once, not again each time its tab comes
			// back into view.
			tab := ""
			if withTabs, ok := c.proc.(cell.WithTabs); ok {
				tab = withTabs.ActiveTab()
			}
			switchedTab := tab != c.lastTab
			c.lastTab = tab
			if state == c.lastState {
				continue
			}
			previous := c.lastState
			c.lastState = state
			// What the mod said was about one state: once the cell leaves
			// it, the word is old.
			if c.said.state != state {
				c.said = said{}
			}
			changed = true
			say := func(text string) {
				if !switchedTab {
					notices = append(notices, notice{text})
				}
			}
			switch state {
			case cell.Replied:
				say(withDetail(c.name+" · "+projectName+" responded", c.takeDetail()))
			case cell.Approve:
				say(withDetail(c.name+" · "+projectName+" approve", c.takeDetail()))
			case cell.Crashed:
				if previous != cell.Stopped {
					say(c.name + " · " + projectName + " crashed")
				}
			}
		}
	}
	e.mu.Unlock()

	for _, n := range notices {
		e.announce(n.text)
	}
	return changed
}

// checkRenames looks for the title of each /rename still waiting. The turn
// ending can't be the cue: /rename is a local command, runs without a turn,
// and an idle agent never leaves the replied state for the watcher to see it
// come back. The title on disk is the only word that always comes.
func (e *Engine) checkRenames() {
	e.mu.Lock()
	var pending []string
	for _, p := range e.projects {
		for _, c := range p.cells {
			if c.pendingRename {
				pending = append(pending, c.id)
			}
		}
	}
	e.mu.Unlock()

	// Outside the lock: adopting the name calls Rename, which locks the
	// mutex again.
	for _, id := range pending {
		_ = e.AdoptAgentName(id)
	}
}

// checkDirectories marks as orphaned the project whose directory has
// disappeared from disk, and takes the mark back when it returns.
//
// The mark lives on the project, not on the cell's last state: that one
// belongs to checkStates, which rewrites it with what the process says on
// the very next tick — and the process, alive in a directory that's gone,
// keeps saying it's working.
func (e *Engine) checkDirectories() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	changed := false
	for _, p := range e.projects {
		info, err := os.Stat(p.path)
		orphan := err != nil || !info.IsDir()
		changed = changed || p.orphan != orphan
		p.orphan = orphan
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
		summary, down := "", false
		if err == nil {
			summary, down = docker.Summary(services), docker.AnyFell(services)
		}
		e.mu.Lock()
		changed = changed || a.p.docker != summary || a.p.dockerDown != down
		a.p.docker, a.p.dockerDown = summary, down
		e.mu.Unlock()
	}
	return changed
}
