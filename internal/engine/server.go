package engine

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/andreluiz/tesseract/internal/protocol"
)

// frameCadence is how often the engine checks whether there's news to send.
// It's also the maximum delay between a keystroke and its echo on screen, so
// it matters more than smoothness: fifty frames per second give ten
// milliseconds of average wait while typing. A frame with no news is neither
// built nor sent, and an idle grid produces none — the cost only shows up
// when there's something to display.
const frameCadence = 20 * time.Millisecond

// ErrEngineAlreadyRunning is the refusal to start a second engine over the
// same socket. It isn't a failure: whoever asked already has what they
// wanted.
var ErrEngineAlreadyRunning = errors.New("an engine is already running")

// Server is the engine's door: a screen connects, sends a key, and gets a
// snapshot back.
type Server struct {
	engine   *Engine
	listener net.Listener
	path     string
	stop     chan struct{}
	once     sync.Once
}

// Serve opens the unix socket. A socket left behind by a dead engine is
// removed; a socket with a live engine on the other end returns an error.
func Serve(e *Engine, path string) (*Server, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if conn, err := net.Dial("unix", path); err == nil {
		conn.Close()
		return nil, ErrEngineAlreadyRunning
	}
	_ = os.Remove(path)

	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	s := &Server{engine: e, listener: listener, path: path, stop: make(chan struct{})}
	go s.accept()
	return s, nil
}

// Stopped closes when someone asked for `tess stop`.
func (s *Server) Stopped() <-chan struct{} { return s.stop }

// Close releases the socket.
func (s *Server) Close() error {
	err := s.listener.Close()
	_ = os.Remove(s.path)
	return err
}

func (s *Server) accept() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

// handle looks after a screen: it reads its requests on one end and pushes
// the snapshot out on the other.
func (s *Server) handle(conn net.Conn) {
	defer conn.Close()

	var writeMu sync.Mutex
	encoder := json.NewEncoder(conn)
	respond := func(kind string, data any) {
		envelope, err := protocol.Pack(kind, data)
		if err != nil {
			return
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = encoder.Encode(envelope)
	}

	done := make(chan struct{})
	go s.pushSnapshots(respond, done)
	defer close(done)

	decoder := json.NewDecoder(conn)
	for {
		var envelope protocol.Message
		if err := decoder.Decode(&envelope); err != nil {
			if err != io.EOF {
				return
			}
			return
		}
		s.handleRequest(envelope, respond)
	}
}

// pushSnapshots sends the whole state as soon as it changes. The screen
// never asks: it draws whatever arrives.
func (s *Server) pushSnapshots(respond func(string, any), done <-chan struct{}) {
	ticker := time.NewTicker(frameCadence)
	defer ticker.Stop()

	respond(protocol.TypeState, s.engine.Snapshot())
	lastSeen := s.engine.Version()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			current := s.engine.Version()
			if current == lastSeen {
				continue
			}
			lastSeen = current
			respond(protocol.TypeState, s.engine.Snapshot())
		}
	}
}

func (s *Server) handleRequest(envelope protocol.Message, respond func(string, any)) {
	fail := func(err error) {
		if err != nil {
			respond(protocol.TypeError, protocol.Error{Message: err.Error()})
		}
	}

	switch envelope.Type {
	case protocol.TypeKey:
		request, err := protocol.Unpack[protocol.Key](envelope)
		if err != nil {
			fail(err)
			return
		}
		fail(s.engine.Key(request))

	case protocol.TypeSize:
		request, err := protocol.Unpack[protocol.Size](envelope)
		if err != nil {
			fail(err)
			return
		}
		fail(s.engine.Resize(request.Cell, request.Cols, request.Rows))

	case protocol.TypeScroll:
		request, err := protocol.Unpack[protocol.Scroll](envelope)
		if err != nil {
			fail(err)
			return
		}
		fail(s.engine.Scroll(request.Cell, request.Delta, request.Live))

	case protocol.TypeCreate:
		request, err := protocol.Unpack[protocol.Create](envelope)
		if err != nil {
			fail(err)
			return
		}
		_, err = s.engine.Create(request)
		fail(err)

	case protocol.TypeKill:
		request, err := protocol.Unpack[protocol.Kill](envelope)
		if err != nil {
			fail(err)
			return
		}
		fail(s.engine.Kill(request.Cell))

	case protocol.TypeRename:
		request, err := protocol.Unpack[protocol.Rename](envelope)
		if err != nil {
			fail(err)
			return
		}
		fail(s.engine.AutoRename(request.Cell))

	case protocol.TypeTab:
		request, err := protocol.Unpack[protocol.Tab](envelope)
		if err != nil {
			fail(err)
			return
		}
		fail(s.engine.SwitchTab(request.Cell, request.Step))

	case protocol.TypeResume:
		request, err := protocol.Unpack[protocol.Resume](envelope)
		if err != nil {
			fail(err)
			return
		}
		fail(s.engine.Resume(request.Cell))

	case protocol.TypePrompt:
		request, err := protocol.Unpack[protocol.Prompt](envelope)
		if err != nil {
			fail(err)
			return
		}
		fail(s.engine.Prompt(request.Cell, request.Text))

	case protocol.TypeEditor:
		request, err := protocol.Unpack[protocol.Editor](envelope)
		if err != nil {
			fail(err)
			return
		}
		fail(s.engine.OpenInEditor(request.Project))

	case protocol.TypeGoToLine:
		request, err := protocol.Unpack[protocol.GoToLine](envelope)
		if err != nil {
			fail(err)
			return
		}
		fail(s.engine.GoToLine(request.Cell, request.Line))

	case protocol.TypeSearch:
		request, err := protocol.Unpack[protocol.Search](envelope)
		if err != nil {
			fail(err)
			return
		}
		matches, err := s.engine.Search(request.Cell, request.Term)
		if err != nil {
			fail(err)
			return
		}
		response := protocol.Matches{Cell: request.Cell, Term: request.Term}
		for _, match := range matches {
			response.Lines = append(response.Lines, protocol.Match{Line: match.Line, Text: match.Text})
		}
		respond(protocol.TypeMatches, response)

	case protocol.TypeDocker:
		request, err := protocol.Unpack[protocol.Docker](envelope)
		if err != nil {
			fail(err)
			return
		}
		// The panel is a slow request: bringing up a service pulls an image.
		// It leaves the socket queue so the screen doesn't freeze while that
		// happens.
		go func() {
			services, err := s.engine.Docker(request)
			if err != nil {
				fail(err)
				return
			}
			respond(protocol.TypeServices, services)
		}()

	case protocol.TypeComplete:
		request, err := protocol.Unpack[protocol.Complete](envelope)
		if err != nil {
			fail(err)
			return
		}
		respond(protocol.TypeCompleted, Complete(request))

	case protocol.TypeScreen:
		request, err := protocol.Unpack[protocol.Screen](envelope)
		if err != nil {
			fail(err)
			return
		}
		s.engine.SwitchScreen(request.Screen)

	case protocol.TypeStatus:
		respond(protocol.TypeSummary, protocol.Summary{Text: s.engine.Summary()})

	case protocol.TypeStop:
		respond(protocol.TypeSummary, protocol.Summary{Text: "engine shut down"})
		s.once.Do(func() { close(s.stop) })
	}
}
