package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/andreluiz/tesseract/internal/engine"
	"github.com/andreluiz/tesseract/internal/protocol"
	"github.com/andreluiz/tesseract/internal/screen"
)

// These two commands are the mod's side of the link: a mod has no socket of
// its own, only the right to run a program, so the program it runs is this
// one. Neither starts an engine — a cell only exists inside one.

// ownCell is the cell this process was born in. It comes from the
// environment the engine gave the agent, never from what the caller says:
// a report can only ever speak for the cell it runs in.
func ownCell() (cell, tab string, err error) {
	cell = os.Getenv("TESSERACT_CELL_ID")
	if cell == "" {
		return "", "", errors.New("not inside a Tesseract cell")
	}
	// Claude Code's background sessions are born from the cell's process and
	// inherit its environment, mods included, but they aren't what the cell
	// shows: one speaking would rename the cell's conversation after itself
	// and take the prompts meant for the session on screen.
	if os.Getenv("CLAUDE_CODE_SESSION_KIND") == "bg" {
		return "", "", errors.New("a background session speaks for no cell")
	}
	return cell, os.Getenv("TESSERACT_TAB"), nil
}

// sendReport tells the engine one thing about the session it runs in, and
// waits to hear whether the engine took it.
func sendReport(args []string) error {
	if len(args) != 1 {
		return errors.New(`usage: ts report '{"kind":"state","state":"working"}'`)
	}
	var report protocol.Report
	decoder := json.NewDecoder(strings.NewReader(args[0]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return fmt.Errorf("unreadable report: %w", err)
	}
	var err error
	if report.Cell, report.Tab, err = ownCell(); err != nil {
		return err
	}

	client, err := screen.Connect(engine.SocketPath())
	if err != nil {
		return errors.New("engine not running")
	}
	defer client.Close()
	if err := client.Send(protocol.TypeReport, report); err != nil {
		return err
	}
	for {
		envelope, err := client.Receive()
		if err != nil {
			return err
		}
		switch envelope.Type {
		case protocol.TypeSummary:
			return nil
		case protocol.TypeError:
			refusal, _ := protocol.Unpack[protocol.Error](envelope)
			return errors.New(refusal.Message)
		}
	}
}

// holdLink stays connected for as long as the mod lives, printing one JSON
// line for each thing the engine hands the cell. The mod reads the lines;
// this process ending is how each side learns the other is gone.
func holdLink() error {
	cell, tab, err := ownCell()
	if err != nil {
		return err
	}
	client, err := screen.Connect(engine.SocketPath())
	if err != nil {
		return errors.New("engine not running")
	}
	defer client.Close()
	if err := client.Send(protocol.TypeLink, protocol.Link{Cell: cell, Tab: tab}); err != nil {
		return err
	}

	lines := json.NewEncoder(os.Stdout)
	for {
		envelope, err := client.Receive()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch envelope.Type {
		case protocol.TypeDeliver:
			delivery, err := protocol.Unpack[protocol.Deliver](envelope)
			if err != nil {
				continue
			}
			if err := lines.Encode(delivery); err != nil {
				return err // the mod stopped reading
			}
		case protocol.TypeError:
			refusal, _ := protocol.Unpack[protocol.Error](envelope)
			return errors.New(refusal.Message)
		}
	}
}
