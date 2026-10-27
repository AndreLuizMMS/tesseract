package cell

import "fmt"

// WithDocs is a cell that can put one markdown file in front of the user
// without the user picking it from a list.
type WithDocs interface {
	OpenDoc(path string) error
}

// OpenDoc reads the file, whatever the tab was showing before.
func (m *Md) OpenDoc(path string) error {
	m.openFile(path)
	m.signal()
	return nil
}

// OpenDoc switches the session to its markdown tab, already reading the file.
func (s *Session) OpenDoc(path string) error {
	const tab = "md"
	s.mu.Lock()
	found := -1
	for i, name := range s.tabs {
		if name == tab {
			found = i
		}
	}
	s.mu.Unlock()
	if found < 0 {
		return fmt.Errorf("the session has no md tab")
	}
	if err := s.openTab(tab, ""); err != nil {
		return err
	}

	s.mu.Lock()
	s.active = found
	reader, _ := s.open[tab].(WithDocs)
	s.mu.Unlock()
	if reader == nil {
		return fmt.Errorf("The md tab is still coming up. Try again in a moment.")
	}
	if err := reader.OpenDoc(path); err != nil {
		return err
	}
	s.notify()
	return nil
}
