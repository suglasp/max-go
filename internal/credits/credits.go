// credits.go : renders the M.A.X. end credits into an 8-bit buffer.
// Suglasp
// github.com/suglasp/max-go
//
// Licensed under the MIT License. See MIT.md file in the project root for full license information.
//
// Package credits provides the original scrolling credits as a buffer component.
package credits

import (
	"github.com/suglasp/max-go/internal/fonts"
	"github.com/suglasp/max-go/internal/gfx"
)

const (
	screenW = 640
	screenH = 480

	columnW = 450
	columnH = 230
	columnX = (screenW - columnW) / 2
	columnY = (screenH - columnH) / 2

	lineBoxH    = 18
	lineSpacing = 20

	scrollRate  = 9
	scrollSpeed = 100

	transparent = 0x80
)

// LineKind identifies how one credits row is drawn.
type LineKind int

const (
	LineSeparator LineKind = iota
	LineTitle
	LineText
)

// Line is one resolved credits row.
type Line struct {
	Kind LineKind
	Text string
}

// Screen is the pure scrolling credits component.
type Screen struct {
	bg                        *gfx.Buffer
	font                      *fonts.Font
	lines                     []Line
	titleColor, textColor     int
	offset                    int
	elapsedMS                 uint64
	done                      bool
	lineScratch, blankScratch *gfx.Buffer
}

// DefaultLines returns the original M.A.X. credits content in order.
//
// C++: menu_credits_lines (menu.cpp)
func DefaultLines(resolve func(key string) string) []Line {
	if resolve == nil {
		resolve = func(key string) string { return key }
	}
	title := func(key string) Line { return Line{Kind: LineTitle, Text: resolve(key)} }
	text := func(s string) Line { return Line{Kind: LineText, Text: s} }
	sep := func() Line { return Line{Kind: LineSeparator} }

	return []Line{
		title("0a6f"),
		text("Ali Atabek"),
		sep(),
		title("a980"),
		text("Ali Atabek"),
		text("Paul Kellner"),
		sep(),
		title("095f"),
		text("Ali Atabek"),
		text("Paul Kellner"),
		text("Gus Smedstad"),
		sep(),
		title("f482"),
		text("Dave Boulanger"),
		sep(),
		title("80f9"),
		title("8bc0"),
		text("Gus Smedstad"),
		sep(),
		title("c663"),
		text("Dave Boulanger"),
		text("Bernie Weir"),
		sep(),
		title("befb"),
		text("Darren Monahan"),
		sep(),
		title("662e"),
		text("Tony Postma"),
		text("Arlene C. Somers"),
		sep(),
		title("41cd"),
		text("Chris Regalado"),
		text("Mark Bergo"),
		text("Arlene C. Somers"),
		text("Mike Dean"),
		sep(),
		title("7891"),
		text("Tony Postma"),
		sep(),
		title("c82a"),
		text("Cheryl Austin"),
		sep(),
		title("68e0"),
		text("Arlene C. Somers"),
		sep(),
		title("2519"),
		text("Ali Atabek"),
		text("Mark Bergo"),
		sep(),
		title("8850"),
		text("Ali Atabek"),
		text("Paul Kellner"),
		text("Steve Perrin"),
		text("Gus Smedstad"),
		sep(),
		title("9ad1"),
		text("Charles Deenen"),
		sep(),
		title("0fd1"),
		text("Steve Perrin"),
		sep(),
		title("fe3d"),
		text("Paul Kellner"),
		text("Tony Postma"),
		text("Amy Mitchell"),
		text("Laura Mitchell"),
		sep(),
		title("936a"),
		text("Kelly Newcomb"),
		sep(),
		title("1331"),
		text("Tony Postma"),
		sep(),
		title("e06f"),
		text("Chad Allison"),
		sep(),
		title("919b"),
		text("Colin Totman"),
		sep(),
		title("c35a"),
		text("Cory Nelson"),
		sep(),
		title("0674"),
		text("Amy Mitchell"),
		text("Chris Peak"),
		text("Quinn Summers"),
		text("Doug Avery"),
		text("Steve McLafferty"),
		text("Erick Lujan"),
		text("Steve Victory"),
		text("James Dunn"),
		text("Henry Kahng"),
		text("Bill Field"),
		text("Steve Reed"),
		text("John Stavros"),
		text("Steve Baldoni"),
		text("Richard Barker"),
		text("Tony Bland"),
		text("Larry Smith"),
		text("Daniel Huffman"),
		text("Arlen Nydam"),
		text("Charles Crail"),
		text("Rene Hakiki"),
		text("Jon Atabek"),
		text("Robert Wood"),
		sep(),
		title("498a"),
		text("Julian Ridley"),
		sep(),
		title("6fbd"),
		text("JBI, Los Angeles"),
		sep(),
		title("364b"),
		text("Gary Burke"),
		text("Sylvie Nguyen"),
		text("Ed Robales"),
		sep(),
		title("a8a5"),
		text("Bill Delk"),
		text("Aaron J. Meyers"),
		sep(),
		title("0853"),
		text("John Werner"),
		sep(),
		title("09fb"),
		text("Dan Forsyth"),
		text("John Parker"),
		text("Aaron Olaiz"),
		text("Derek Gibbs"),
		text("Phuong Nguyen"),
		text("Marc Duran"),
		sep(),
		title("51c9"),
		text("Paul Edelstein"),
		sep(),
		title("9dc7"),
		text("Bill Stoudt"),
		sep(),
		title("f6a7"),
		text("Jim Veevaert"),
		sep(),
		title("abc4"),
		text("Dean Schulte"),
		sep(),
		title("0ae1"),
		text("Julie Roether"),
		sep(),
		title("8ead"),
		text("Nicole Pelerine"),
		text("Brian Cummings"),
		sep(),
		title("fab6"),
		text("Charles Deenen"),
		sep(),
		title("1795"),
		text("Chris Borders"),
		sep(),
		title("b291"),
		text("Craig Duman"),
		sep(),
		title("31f7"),
		text("Sergio Bustamante"),
		text("Chris Borders"),
		text("Craig Duman"),
		text("Doug Rappaport"),
		sep(),
		title("b23a"),
		text("Brian Luzietti"),
		sep(),
		title("f0f9"),
		text("Rick Jackson"),
		sep(),
		title("d040"),
		text("Digital Brothers"),
		sep(),
		title("9964"),
		text("Gregory G. Allen"),
		text("Larry Peacock"),
		sep(),
		title("177b"),
		text("Craig Duman"),
		sep(),
		title("67e6"),
		text("James Doyle"),
		text("Apple's Animation"),
		sep(),
		title("c5af"),
		text("Albert Olsen for Four Bars Intertainment"),
		sep(),
		title("9376"),
		text("David Farmer"),
		text("Harry Cohen"),
		text("Jeffrey R. Whitcher"),
		text("Elisabeth Flaum"),
		sep(),
		title("387d"),
		text("Ken Teaney"),
		text("Mashall Garlington"),
		sep(),
		title("99e0"),
		text("Charles Deenen"),
		sep(),
		title("12f1"),
		text("Doug Rappaport"),
		sep(),
		title("85c8"),
		text("Doug Rappaport"),
		text("Sergio Bustamante"),
		sep(),
		title("f813"),
		title("11ae"),
		title("06d1"),
		title("b2a8"),
		sep(),
		sep(),
		sep(),
		sep(),
		title("7a22"),
		title("efb3"),
		title("fda7"),
		title("ab0a"),
		title("163c"),
		title("3ce2"),
		title("d126"),
		title("00a5"),
		title("7bd6"),
		sep(),
		sep(),
		sep(),
	}
}

// New builds the scrolling credits component.
func New(bg *gfx.Buffer, font *fonts.Font, lines []Line, titleColor, textColor int) *Screen {
	cp := append([]Line(nil), lines...)
	return &Screen{
		bg:           bg,
		font:         font,
		lines:        cp,
		titleColor:   titleColor,
		textColor:    textColor,
		lineScratch:  gfx.New(columnW, lineBoxH),
		blankScratch: gfx.New(columnW, lineBoxH),
	}
}

// Advance scrolls by elapsed wall clock time and reports whether the screen is still active.
// The offset tracks total elapsed time at scrollRate/scrollSpeed (9 px per 100 ms),
// so it advances a pixel or two per frame instead of jumping 9 px every 100 ms.
func (s *Screen) Advance(elapsedMS uint64) bool {
	if s.done {
		return false
	}
	s.elapsedMS += elapsedMS
	s.offset = int(s.elapsedMS * scrollRate / scrollSpeed)
	if s.offset >= s.finishOffset() {
		s.done = true
		return false
	}
	return true
}

// Draw composites the background and clipped scrolling text into dst.
func (s *Screen) Draw(dst *gfx.Buffer) {
	if dst == nil {
		return
	}
	if s.bg != nil {
		copyClipped(dst, s.bg, 0, 0, false, 0)
	}
	if len(s.lines) == 0 {
		return
	}
	for i, line := range s.lines {
		y := columnY + columnH - s.offset + i*lineSpacing
		if y >= columnY+columnH || y+lineBoxH <= columnY {
			continue
		}
		s.drawLine(dst, line, y)
	}
}

// finishOffset returns the scroll offset where the last line has left the view.
func (s *Screen) finishOffset() int {
	return columnH + len(s.lines)*lineSpacing
}

// drawLine centers and renders one credits row at the supplied column y.
func (s *Screen) drawLine(dst *gfx.Buffer, line Line, y int) {
	if s.font == nil || line.Kind == LineSeparator || line.Text == "" {
		return
	}
	s.lineScratch.Copy(s.blankScratch)
	s.lineScratch.Fill(transparent)

	color := s.textColor
	if line.Kind == LineTitle {
		color = s.titleColor
	}
	tw := s.font.Width(line.Text)
	x := (columnW - tw) / 2
	if x < 0 {
		x = 0
	}
	textY := (lineBoxH - s.font.Height) / 2
	if textY < 0 {
		textY = 0
	}
	s.font.Blit(s.lineScratch.Sub(x, textY, columnW-x, min(s.font.Height, lineBoxH-textY)), line.Text, columnW-x, fonts.FlagOutline|color)
	copyClipped(dst, s.lineScratch, columnX, y, true, transparent)
}

// copyClipped copies src into dst at x/y, clipping to dst and honoring transparency.
func copyClipped(dst, src *gfx.Buffer, x, y int, transparentCopy bool, transparentColor uint8) {
	for sy := 0; sy < src.H; sy++ {
		dy := y + sy
		if dy < 0 || dy >= dst.H {
			continue
		}
		if dy < columnY && transparentCopy || dy >= columnY+columnH && transparentCopy {
			continue
		}
		srow := src.Row(sy)
		for sx, c := range srow {
			dx := x + sx
			if dx < 0 || dx >= dst.W {
				continue
			}
			if transparentCopy {
				if dx < columnX || dx >= columnX+columnW || c == transparentColor {
					continue
				}
			}
			dst.Set(dx, dy, c)
		}
	}
}
