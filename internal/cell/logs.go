package cell

import (
	"fmt"
	"time"

	"github.com/andreluiz/tesseract/internal/docker"
)

func init() {
	Register(Descriptor{
		Type:        "logs",
		Order:       40,
		TargetLabel: "SERVICE",
	}, func() Cell { return &Logs{} })
}

// reattachInterval is how often the cell tries to follow a service that
// isn't up again.
const reattachInterval = 3 * time.Second

// Logs is the live log of a compose service. Read-only.
type Logs struct {
	Process
	file    string
	service string
	program string
	args    []string
	stop    chan struct{}
}

func (l *Logs) Spawn(cfg Config) error {
	if cfg.Target == "" {
		return fmt.Errorf("the logs cell needs to know which service to follow")
	}
	l.file = docker.Detect(cfg.Directory)
	if l.file == "" {
		return fmt.Errorf("the project has no compose file at its root")
	}
	l.service = cfg.Target
	l.program, l.args = docker.LogCommand(l.file, l.service)
	l.stop = make(chan struct{})

	if err := l.start(cfg, l.program, l.args, terminalEnvironment()); err != nil {
		return err
	}
	go l.reattachWhenUp()
	return nil
}

// reattachWhenUp is what makes the cell wait for the service: while it's
// down the log dies on its own, and the cell keeps trying until it comes
// back.
func (l *Logs) reattachWhenUp() {
	clock := time.NewTicker(reattachInterval)
	defer clock.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-clock.C:
			l.mu.Lock()
			crashed := l.finished && !l.dying
			l.mu.Unlock()
			if !crashed {
				continue
			}
			_ = l.reconnect(l.program, l.args, terminalEnvironment())
		}
	}
}

// State: a logs cell for a stopped service is a cell waiting, not a crashed
// cell — the service coming back is normal, and it reattaches on its own.
func (l *Logs) State() State {
	if state := l.Process.State(); state == Crashed {
		return Stopped
	} else {
		return state
	}
}

func (l *Logs) States() []State {
	return []State{Working, Stopped, Orphaned}
}

// Key does nothing: a logs cell is read-only.
func (l *Logs) Key(Keystroke) error { return nil }

func (l *Logs) Kill() error {
	select {
	case <-l.stop:
	default:
		close(l.stop)
	}
	return l.Process.Kill()
}
