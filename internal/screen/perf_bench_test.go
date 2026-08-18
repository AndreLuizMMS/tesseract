package screen

import (
	"strconv"
	"strings"
	"testing"

	"github.com/andreluiz/tesseract/internal/keyboard"
	"github.com/andreluiz/tesseract/internal/protocol"
)

func loadState(projects, cells, columns, lines int) protocol.State {
	var e protocol.State
	for p := range projects {
		proj := protocol.Project{
			ID: "p" + strconv.Itoa(p), Path: "/home/user/projeto" + strconv.Itoa(p),
			Name: "project" + strconv.Itoa(p), Color: p, HasCompose: true, Docker: "3/4",
		}
		for c := range cells {
			cell := protocol.Cell{
				ID: "c" + strconv.Itoa(p) + "-" + strconv.Itoa(c), Type: "session",
				Name: "item " + strconv.Itoa(c), State: "working", Live: true,
			}
			for l := range lines {
				cell.Lines = append(cell.Lines,
					"\x1b[38;2;120;200;180m"+strings.Repeat("x", columns/2)+
						"\x1b[0m \x1b[1mlinha "+strconv.Itoa(l)+"\x1b[0m")
			}
			proj.Cells = append(proj.Cells, cell)
		}
		e.Projects = append(e.Projects, proj)
	}
	return e
}

func BenchmarkDrawGrid(b *testing.B) {
	state := loadState(3, 4, 100, 24)
	b.ReportAllocs()
	for b.Loop() {
		_ = Draw(state, Focus{}, keyboard.Browse, 200, 55, "")
	}
}

func BenchmarkArrange(b *testing.B) {
	state := loadState(3, 4, 100, 24)
	b.ReportAllocs()
	for b.Loop() {
		_ = Arrange(state, Focus{}, 200, 55)
	}
}
