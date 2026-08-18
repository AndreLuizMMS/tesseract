package screen

import (
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/andreluiz/tesseract/internal/theme"
)

// The opening is the only place in Tesseract where motion exists for its own
// sake. It lasts about a second and runs while the engine is being looked
// for, so its time is time that would pass anyway — no wait is invented.
//
// The brand doesn't appear ready-made: it draws itself. The back square
// first, the front one on top, each one stroke by stroke, with the pen tip
// lit in phosphor. The tessera lights up at the end, the whole symbol bursts
// in one frame and settles. Only then does the name open, letter by letter.
//
// None of this is glow, scanline or glitch — the terminal emits no light and
// the brand rule still holds. What moves here is the order in which things
// exist, not their texture.
const (
	strokeStep   = 22 * time.Millisecond
	ignitionStep = 90 * time.Millisecond
	burstStep    = 130 * time.Millisecond
	brandPause   = 260 * time.Millisecond
	letterStep   = 62 * time.Millisecond
	namePause    = 200 * time.Millisecond
	countStep    = 170 * time.Millisecond
)

// strokePath is the order in which the drawing is born: the whole back
// square, then the front one on top. Each pair is a row and column inside
// the 7×5 symbol. Each square's path is that of a pen that never lifts: top
// left to right, down the right side, back along the bottom, up the left
// side.
var strokePath = [][2]int{
	// back square
	{1, 1}, {1, 2}, {1, 3}, {1, 4}, {1, 5}, {1, 6},
	{2, 6}, {3, 6},
	{4, 6}, {4, 5}, {4, 4}, {4, 3}, {4, 2}, {4, 1},
	{3, 1}, {2, 1},
	// front square
	{0, 0}, {0, 1}, {0, 2}, {0, 3}, {0, 4}, {0, 5},
	{1, 5}, {2, 5},
	{3, 5}, {3, 4}, {3, 3}, {3, 2}, {3, 1}, {3, 0},
	{2, 0}, {1, 0},
}

// tesseraPosition is where the brand lights up.
var tesseraPosition = [2]int{2, 3}

// openingHeight is the whole block: the symbol's five lines, one blank line
// and the three lines of the tally.
const openingHeight = 9

// RequiredWidth is the width, in columns, of the longest line the opening
// ever draws. Below that the terminal wraps the line, the animation's fixed
// cursor-up miscounts, and the screen is left with leftover garbage — so
// whoever decides to animate needs to check against this first.
func RequiredWidth() int {
	signature := []string{"", theme.Name, theme.Tagline, "", theme.Version}
	biggest := 0
	for i := range theme.Symbol {
		side := ""
		if i < len(signature) {
			side = signature[i]
		}
		if width := len([]rune("   " + theme.Symbol[i] + "    " + side)); width > biggest {
			biggest = width
		}
	}
	return biggest
}

// Opening is the startup animation. It always paints the whole block and
// rewrites it in place, so it leaves no trail in the scrollback.
type Opening struct {
	output  io.Writer
	animate bool
	// lit tracks which positions of the symbol have already been drawn.
	lit map[[2]int]bool
	// name is how much of the wordmark has opened, in letters.
	name int
	// signed turns on the tagline and version.
	signed bool
	// burst paints the whole symbol in phosphor for one frame.
	burst bool
	// tally is the engine's lines, which arrive after the connection.
	tally []string
	// drawn says whether the block already occupies the screen and can be
	// rewritten in place.
	drawn bool
}

// NewOpening prepares the animation. With animate false it becomes the usual
// static banner — what happens without a real terminal, or with
// TESSERACT_SEM_ABERTURA set in the environment.
func NewOpening(output io.Writer, animate bool) *Opening {
	return &Opening{output: output, animate: animate, lit: map[[2]int]bool{}}
}

// Build draws the brand assembling itself and the name opening. Returns as
// soon as the whole block is on screen.
func (a *Opening) Build() {
	if !a.animate {
		a.lightAll()
		a.paint()
		return
	}

	// The stroke: the pen tip is the last point of each frame.
	for i, point := range strokePath {
		a.lit[point] = true
		a.paintWithTip(&strokePath[i])
		time.Sleep(strokeStep)
	}

	// The ignition: the tessera lights up on its own, and the whole symbol
	// bursts in a single frame before settling.
	a.lit[tesseraPosition] = true
	for range 3 {
		a.paintWithTip(&tesseraPosition)
		time.Sleep(ignitionStep)
	}
	a.burst = true
	a.paint()
	time.Sleep(burstStep)
	a.burst = false
	a.paint()

	// The finished brand holds by itself for a moment before the name comes
	// in. It's the pause that separates the opening's two halves — without
	// it, the wordmark would run over the drawing that just came to life.
	time.Sleep(brandPause)

	// The name: one letter at a time, the last one always lit.
	for a.name < len([]rune(theme.Name)) {
		a.name++
		a.paint()
		time.Sleep(letterStep)
	}
	a.signed = true
	a.paint()
	time.Sleep(namePause)
}

// Tally closes the opening with what the engine returned, one line at a
// time.
func (a *Opening) Tally(lines []string) {
	if !a.animate {
		a.tally = lines
		a.paint()
		return
	}
	for _, line := range lines {
		a.tally = append(a.tally, line)
		a.paint()
		time.Sleep(countStep)
	}
}

// lightAll puts the block in its final state, with no animation at all.
func (a *Opening) lightAll() {
	for _, point := range strokePath {
		a.lit[point] = true
	}
	a.lit[tesseraPosition] = true
	a.name = len([]rune(theme.Name))
	a.signed = true
}

func (a *Opening) paint() { a.paintWithTip(nil) }

// paintWithTip writes the whole block at once. The tip, when it exists, is
// the position that just came to life and comes out in phosphor — it's what
// gives the sense of a pen running across the drawing.
func (a *Opening) paintWithTip(tip *[2]int) {
	var frame strings.Builder
	if a.drawn {
		// Moves up to the top of the block to rewrite in place. Without
		// this the animation would turn into a cascade of copies down the
		// scrollback.
		frame.WriteString("\x1b[" + strconv.Itoa(openingHeight) + "A")
	}
	a.drawn = true

	for _, line := range a.blockLines(tip) {
		frame.WriteString("\x1b[2K" + line + "\n")
	}
	_, _ = io.WriteString(a.output, frame.String())
}

// blockLines builds the current frame's nine lines.
func (a *Opening) blockLines(tip *[2]int) []string {
	signature := []string{"", a.wordmark(), a.tagline(), "", a.version()}
	lines := make([]string, 0, openingHeight)

	for i := range theme.Symbol {
		side := ""
		if i < len(signature) {
			side = signature[i]
		}
		lines = append(lines, strings.TrimRight("   "+a.symbolLine(i, tip)+"    "+side, " "))
	}

	lines = append(lines, "")
	for i := range 3 {
		if i < len(a.tally) {
			lines = append(lines, "   "+a.tally[i])
			continue
		}
		lines = append(lines, "")
	}
	return lines
}

// symbolLine paints one line of the drawing in its current state: whatever
// has already come to life shows in the color of the square it belongs to,
// whatever hasn't is a space.
func (a *Opening) symbolLine(line int, tip *[2]int) string {
	var out strings.Builder
	phosphor := theme.Paint(theme.BrandPhosphor, "").Bold(true)
	for column, r := range []rune(theme.Symbol[line]) {
		position := [2]int{line, column}
		paint, has := theme.SymbolColor(line, column)
		switch {
		case !has:
			out.WriteRune(r)
		case !a.lit[position]:
			out.WriteRune(' ')
		case a.burst || (tip != nil && *tip == position):
			out.WriteString(phosphor.Render(string(r)))
		default:
			out.WriteString(paint.Render(string(r)))
		}
	}
	return out.String()
}

// wordmark opens the name letter by letter, with the one that just arrived
// lit.
func (a *Opening) wordmark() string {
	letters := []rune(theme.Name)
	if a.name <= 0 {
		return ""
	}
	open, lit := string(letters[:max(a.name-1, 0)]), ""
	if a.name <= len(letters) {
		lit = string(letters[a.name-1])
	}
	bright := theme.Paint(theme.FgBright, "").Bold(true)
	if a.name >= len(letters) {
		return bright.Render(string(letters))
	}
	return bright.Render(open) + theme.Paint(theme.BrandPhosphor, "").Bold(true).Render(lit)
}

func (a *Opening) tagline() string {
	if !a.signed {
		return ""
	}
	return theme.Paint(theme.FgMuted, "").Render(theme.Tagline)
}

func (a *Opening) version() string {
	if !a.signed {
		return ""
	}
	return theme.Paint(theme.FgFaint, "").Render(theme.Version)
}

// EngineLine formats one line of the tally: the arrow in cyan, because it's
// structure, and the text in the usual tone.
func EngineLine(text string) string {
	return theme.Paint(theme.FluxCore, "").Render(">") + " " + theme.Paint(theme.FgMuted, "").Render(text)
}
