package cell

import (
	"fmt"

	"github.com/andreluiz/tesseract/internal/docker"
)

func init() {
	Register(Descriptor{
		Type:        "shell",
		Order:       45,
		TargetLabel: "SERVICE",
	}, func() Cell { return &Shell{} })
}

// Shell is a shell running inside a compose service's container. Unlike the
// logs cell it takes the keyboard: it's a terminal in there, not a reading.
type Shell struct {
	Process
}

func (s *Shell) Spawn(cfg Config) error {
	if cfg.Target == "" {
		return fmt.Errorf("the shell cell needs to know which service to enter")
	}
	file := docker.Detect(cfg.Directory)
	if file == "" {
		return fmt.Errorf("the project has no compose file at its root")
	}
	program, args := docker.ShellCommand(file, cfg.Target)
	return s.start(cfg, program, args, terminalEnvironment())
}
