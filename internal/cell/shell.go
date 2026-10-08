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
		return fmt.Errorf("The shell cell needs a service. d opens the Docker panel to pick one.")
	}
	file := docker.Detect(cfg.Directory)
	if file == "" {
		return fmt.Errorf("No compose file at the root of this project.")
	}
	program, args := docker.ShellCommand(file, cfg.Target)
	return s.start(cfg, program, args, terminalEnvironment())
}
