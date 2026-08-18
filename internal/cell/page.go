package cell

import (
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"

	"github.com/andreluiz/tesseract/internal/theme"
)

// The markdown tab draws the file as a page, not as terminal output: the
// cell's whole width, margin on both sides, a bold title and space around
// the blocks. Reading long documentation on a wide screen is the reason
// Tesseract exists.
const (
	// minPageWidth is the least the text needs to still wrap into legible
	// lines.
	minPageWidth = 20
	// pageMargin is the breathing room between the cell's edge and the text.
	pageMargin = 2
)

// buildPageStyle is the theme for the rendered markdown. It depends on the
// width because the horizontal rule spans the whole page.
func buildPageStyle(width int) ansi.StyleConfig {
	style := styles.DarkStyleConfig

	text := func(s string) *string { return &s }
	yes := func() *bool { truth := true; return &truth }
	no := func() *bool { falsehood := false; return &falsehood }
	number := func(n uint) *uint { return &n }

	// The margin is applied out here, along with centering the page.
	style.Document.Margin = number(0)
	style.Document.BlockPrefix = ""
	style.Document.BlockSuffix = "\n"
	style.Document.Color = text(theme.FgDefault)

	// Title: a solid bar spanning the line, like a book chapter. The bar is
	// cyan because a title is document structure, not state.
	style.H1.Prefix = "  "
	style.H1.Suffix = "  "
	style.H1.Color = text(theme.Flux)
	style.H1.BackgroundColor = text(theme.FluxDeep)
	style.H1.Bold = yes()
	style.H1.Upper = yes()
	style.H1.BlockPrefix = "\n"
	style.H1.BlockSuffix = "\n"

	// Sections: a bar on the left instead of markdown's hashes. The
	// hierarchy descends through brightness, not hue — strong cyan, faint
	// cyan, gray.
	style.H2.Prefix = "▌ "
	style.H2.Color = text(theme.Flux)
	style.H2.Bold = yes()
	style.H2.BlockPrefix = "\n"
	style.H2.BlockSuffix = "\n"

	style.H3.Prefix = "▏ "
	style.H3.Color = text(theme.FluxCore)
	style.H3.Bold = yes()
	style.H3.BlockPrefix = "\n"

	style.H4.Prefix = "· "
	style.H4.Color = text(theme.FgBright)
	style.H4.Bold = yes()
	style.H5.Prefix = "· "
	style.H5.Color = text(theme.FgMuted)
	style.H6.Prefix = "· "
	style.H6.Color = text(theme.FgFaint)
	style.H6.Bold = no()

	// Blockquote with a rule on the left, in faded italics.
	style.BlockQuote.IndentToken = text("┃ ")
	style.BlockQuote.Color = text(theme.FgMuted)
	style.BlockQuote.Italic = yes()

	// Rule: one whole line, not eight dashes.
	style.HorizontalRule.Color = text(theme.LineDim)
	style.HorizontalRule.Format = "\n" + strings.Repeat("─", max(width-4, 1)) + "\n"

	// Lists with a round marker and room to breathe.
	style.Item.BlockPrefix = "• "
	style.Item.Color = text(theme.FgDefault)
	style.Enumeration.BlockPrefix = ". "
	style.Enumeration.Color = text(theme.FluxCore)

	// Code: its own background, like a technical book's code box.
	style.Code.Color = text(theme.FluxCore)
	style.Code.BackgroundColor = text(theme.BgRaised)
	style.Code.Prefix = " "
	style.Code.Suffix = " "
	style.CodeBlock.Margin = number(2)
	// No third-party theme: syntax highlighting uses the same palette as
	// the rest.
	style.CodeBlock.Theme = ""
	style.CodeBlock.Chroma = codeHighlight()

	// Legible links, without becoming noise.
	style.Link.Color = text(theme.Flux)
	style.Link.Underline = yes()
	style.LinkText.Color = text(theme.FluxCore)
	style.LinkText.Bold = yes()

	// Table with a thin rule.
	style.Table.CenterSeparator = text("┼")
	style.Table.ColumnSeparator = text("│")
	style.Table.RowSeparator = text("─")

	style.Emph.Italic = yes()
	style.Strong.Bold = yes()
	style.Strong.Color = text(theme.FgBright)
	return style
}

// codeHighlight is the syntax highlighting inside the code block, in
// Tesseract's palette. It follows the same reading as the editor's theme:
// string in dark green, function in blue, keyword in purple, type in
// yellow, number in orange. Phosphor green stays out — a code block would
// repeat the keyboard-ownership color dozens of times.
func codeHighlight() *ansi.Chroma {
	color := func(c string) ansi.StylePrimitive { return ansi.StylePrimitive{Color: &c} }
	bold := func(c string) ansi.StylePrimitive {
		truth := true
		return ansi.StylePrimitive{Color: &c, Bold: &truth}
	}
	italic := func(c string) ansi.StylePrimitive {
		truth := true
		return ansi.StylePrimitive{Color: &c, Italic: &truth}
	}
	background := func(c string) ansi.StylePrimitive { return ansi.StylePrimitive{BackgroundColor: &c} }
	return &ansi.Chroma{
		Text:                color(theme.FgDefault),
		Error:               color(theme.StateDead),
		Comment:             italic(theme.FgFaint),
		CommentPreproc:      color(theme.Flux),
		Keyword:             color(theme.StateOrphan),
		KeywordReserved:     color(theme.StateOrphan),
		KeywordNamespace:    color(theme.StateOrphan),
		KeywordType:         color(theme.AnsiYellow),
		Operator:            color(theme.FgMuted),
		Punctuation:         color(theme.FgMuted),
		Name:                color(theme.FgDefault),
		NameBuiltin:         color(theme.AnsiMagenta),
		NameTag:             color(theme.StateOrphan),
		NameAttribute:       color(theme.Flux),
		NameClass:           bold(theme.AnsiYellow),
		NameConstant:        color(theme.StateBlock),
		NameDecorator:       color(theme.Flux),
		NameException:       color(theme.StateDead),
		NameFunction:        color(theme.StateRead),
		NameOther:           color(theme.FgDefault),
		Literal:             color(theme.StateBlock),
		LiteralNumber:       color(theme.StateBlock),
		LiteralDate:         color(theme.StateBlock),
		LiteralString:       color(theme.BrandCore),
		LiteralStringEscape: color(theme.Flux),
		GenericDeleted:      color(theme.StateDead),
		GenericEmph:         italic(theme.FgDefault),
		GenericInserted:     color(theme.BrandCore),
		GenericStrong:       bold(theme.FgBright),
		GenericSubheading:   color(theme.FluxCore),
		Background:          background(theme.BgSurface),
	}
}

// renderPage draws the markdown as a page: the text fills the cell's whole
// width, with a margin on each side.
func renderPage(raw string, columns int) []string {
	width := pageWidth(columns)
	// Code wider than the page is truncated, never wrapped: a diagram or a
	// terminal screen split in the middle turns into unreadable noise.
	raw = truncateWideCode(raw, width-4)
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(buildPageStyle(width)),
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
	)
	if err != nil {
		return strings.Split(raw, "\n")
	}
	output, err := renderer.Render(raw)
	if err != nil {
		return strings.Split(raw, "\n")
	}

	indent := strings.Repeat(" ", max((columns-width)/2, 0))
	lines := strings.Split(strings.Trim(output, "\n"), "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return lines
}

// pageWidth is the whole cell's width minus the margin on both sides. The
// page fills whatever the screen gives it: on a wide screen, long
// documentation fits with less scrolling.
func pageWidth(columns int) int {
	return max(columns-2*pageMargin, minPageWidth)
}

// truncateWideCode shortens the lines inside code blocks that don't fit the
// page, marking the cut.
func truncateWideCode(raw string, width int) string {
	if width < 10 {
		return raw
	}
	lines := strings.Split(raw, "\n")
	inside := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inside = !inside
			continue
		}
		if !inside {
			continue
		}
		if runes := []rune(line); len(runes) > width {
			lines[i] = string(runes[:width-1]) + "›"
		}
	}
	return strings.Join(lines, "\n")
}
