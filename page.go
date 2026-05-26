// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdftable

import (
	"fmt"
	"strings"

	"github.com/hallelx2/pdftable/internal/pdf"
)

// Page is one page of a PDF document. The interface (not a struct) is
// intentional: it lets us swap implementations later (e.g. for a
// streaming PDF parser) without breaking callers, and it makes the
// API surface easy to mock in tests.
//
// Every accessor (Chars, Lines, Rects, Curves, Objects) walks the
// page content stream from scratch. We do NOT cache between calls
// because:
//
//  1. Callers that need ALL the objects call Objects() once.
//  2. Callers that need just the chars (say, for text extraction)
//     don't pay for the path-painting machinery they aren't using.
//  3. Caching means deciding when to invalidate, which is moot
//     because a Page is immutable from the caller's perspective.
//
// Pages are 1-indexed, matching pdfplumber. Number() returns the
// 1-based index so callers can format error messages without
// re-tracking which Page they were given.
type Page interface {
	// Number returns the page number (1-based).
	Number() int

	// Width and Height return the page's mediabox dimensions in PDF
	// points (1/72 inch), already adjusted for the page's /Rotate
	// entry — so a portrait letter-sized page rotated 90 degrees
	// reports Width=792 Height=612 (landscape), matching what a
	// PDF viewer would display.
	Width() float64
	Height() float64

	// Chars walks the page and returns every positioned glyph. The
	// order is content-stream order — i.e. the order the producer
	// drew them, NOT visual reading order. Downstream layout code
	// (extract_text, find_tables) sorts the chars by position.
	Chars() ([]Char, error)

	// Lines returns every straight-line segment drawn on the page.
	// Each `l` segment in the content stream becomes one Line.
	// Rectangles drawn via `re` are NOT decomposed into four Lines;
	// they're reported through Rects() instead.
	Lines() ([]Line, error)

	// Rects returns every rectangle drawn via the `re` operator.
	// Both stroked and filled rectangles are returned; the Stroke
	// and Fill flags say which.
	Rects() ([]Rect, error)

	// Curves returns every Bezier or composite path that isn't a
	// pure line-segment chain or a single rect.
	Curves() ([]Curve, error)

	// Objects returns Chars + Lines + Rects + Curves in a single
	// walk. Use this when you need all four — it's strictly
	// cheaper than calling each accessor separately because the
	// content stream is parsed exactly once.
	Objects() (Objects, error)
}

// page is the unexported implementation backing the Page interface.
// It wraps a *pdf.Reader (so all four accessors can lazily ReadPage)
// and the 1-based index. We separate the interface from the impl so
// that future implementations (a Page backed by a different PDF
// parser, a Page deserialised from a cached layout) can plug in.
type page struct {
	doc    *document
	number int
}

func (p *page) Number() int { return p.number }

// mediaBoxAndRotation looks up the page's mediabox and rotate value.
// We do this on demand because Width/Height callers don't necessarily
// want to pay the cost of resolving the page's content stream.
func (p *page) mediaBoxAndRotation() (mb [4]float64, rotate int, err error) {
	info, err := p.doc.reader.ReadPage(p.number)
	if err != nil {
		return mb, 0, fmt.Errorf("%w: %v", ErrInvalidPDF, err)
	}
	return info.MediaBox, info.Rotate, nil
}

func (p *page) Width() float64 {
	mb, rot, err := p.mediaBoxAndRotation()
	if err != nil {
		return 0
	}
	w := mb[2] - mb[0]
	h := mb[3] - mb[1]
	// Swap dimensions for 90/270 rotations: viewer shows the page
	// landscape so the "width" the caller cares about is the height
	// of the rotated mediabox.
	if rot == 90 || rot == 270 {
		return h
	}
	return w
}

func (p *page) Height() float64 {
	mb, rot, err := p.mediaBoxAndRotation()
	if err != nil {
		return 0
	}
	w := mb[2] - mb[0]
	h := mb[3] - mb[1]
	if rot == 90 || rot == 270 {
		return w
	}
	return h
}

// collector is the Sink implementation that gathers events from one
// interpreter run. It buffers everything; we don't stream because the
// public API returns slices. For very large pages this could be a
// memory concern — a future optimisation could expose a callback-based
// alternative, but it's not justified at this stage when typical PDFs
// have a few hundred glyphs per page.
type collector struct {
	chars  []Char
	lines  []Line
	rects  []Rect
	curves []Curve
}

func (c *collector) EmitChar(ev pdf.CharEvent) {
	c.chars = append(c.chars, Char{
		Text:     ev.Text,
		X0:       ev.X0,
		Y0:       ev.Y0,
		X1:       ev.X1,
		Y1:       ev.Y1,
		FontName: ev.FontName,
		FontSize: ev.FontSize,
		Upright:  ev.Upright,
		Advance:  ev.Advance,
	})
}

func (c *collector) EmitPath(ev pdf.PathEvent) {
	// Classify the path. If it's exactly one `re` segment, emit a
	// Rect. If every non-h segment is `l`, emit a Line per `l`. Else
	// emit a Curve summarising the path. This is the same partition
	// pdfplumber uses to populate page.lines / page.rects / page.curves.
	if isSingleRect(ev.Segments) {
		s := ev.Segments[0]
		c.rects = append(c.rects, Rect{
			X0: s.X, Y0: s.Y,
			X1: s.X + s.W, Y1: s.Y + s.H,
			Stroke: ev.Stroke, Fill: ev.Fill, Width: ev.LineWidth,
		})
		return
	}
	if onlyLines(ev.Segments) {
		var prev [2]float64
		havePrev := false
		for _, s := range ev.Segments {
			switch s.Op {
			case "m":
				prev = [2]float64{s.X, s.Y}
				havePrev = true
			case "l":
				if havePrev && ev.Stroke {
					c.lines = append(c.lines, Line{
						X0: prev[0], Y0: prev[1],
						X1: s.X, Y1: s.Y,
						Stroke: ev.Stroke, Width: ev.LineWidth,
					})
				}
				prev = [2]float64{s.X, s.Y}
				havePrev = true
			case "h":
				// closepath: connect back to subpath start. Most
				// generators only `h` their rectangles, which we
				// already caught above; ignore here.
			}
		}
		return
	}
	// Generic curve: flatten endpoints in order.
	var pts [][2]float64
	for _, s := range ev.Segments {
		switch s.Op {
		case "m", "l", "h":
			pts = append(pts, [2]float64{s.X, s.Y})
		case "c", "y":
			pts = append(pts, [2]float64{s.X3, s.Y3})
		case "v":
			pts = append(pts, [2]float64{s.X3, s.Y3})
		case "re":
			pts = append(pts,
				[2]float64{s.X, s.Y},
				[2]float64{s.X + s.W, s.Y},
				[2]float64{s.X + s.W, s.Y + s.H},
				[2]float64{s.X, s.Y + s.H},
			)
		}
	}
	c.curves = append(c.curves, Curve{
		Points: pts, Stroke: ev.Stroke, Fill: ev.Fill, Width: ev.LineWidth,
	})
}

func isSingleRect(segs []pdf.PathSeg) bool {
	if len(segs) == 0 {
		return false
	}
	if segs[0].Op != "re" {
		return false
	}
	// A bare `re` is one segment; some generators emit `re h` to
	// explicitly close. Tolerate either.
	for i := 1; i < len(segs); i++ {
		if segs[i].Op != "h" {
			return false
		}
	}
	return true
}

func onlyLines(segs []pdf.PathSeg) bool {
	for _, s := range segs {
		switch s.Op {
		case "m", "l", "h":
			// ok
		default:
			return false
		}
	}
	return true
}

func (p *page) run() (*collector, error) {
	info, err := p.doc.reader.ReadPage(p.number)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPDF, err)
	}
	col := &collector{}
	if len(info.Content) == 0 {
		return col, nil
	}
	// Initial CTM: PDF default coordinate system has origin at
	// bottom-left of the mediabox. We translate so (0,0) on the
	// content stream lines up with the mediabox lower-left corner,
	// which is what pdfplumber and PDF viewers report.
	ctm := pdf.IdentityMatrix
	if info.MediaBox != [4]float64{} {
		ctm = pdf.Matrix{1, 0, 0, 1, -info.MediaBox[0], -info.MediaBox[1]}
	}
	// Rotation: PDF 1.7 §14.8.2 — rotate values are clockwise; we
	// match what a viewer would display by pre-rotating the CTM so
	// the rotated content appears axis-aligned to the caller.
	switch info.Rotate {
	case 90:
		ctm = pdf.Mult(pdf.Matrix{0, -1, 1, 0, -info.MediaBox[1], info.MediaBox[2]}, ctm)
	case 180:
		ctm = pdf.Mult(pdf.Matrix{-1, 0, 0, -1, info.MediaBox[2], info.MediaBox[3]}, ctm)
	case 270:
		ctm = pdf.Mult(pdf.Matrix{0, 1, -1, 0, info.MediaBox[3], -info.MediaBox[0]}, ctm)
	}
	interp := pdf.NewInterpreter(ctm, col)
	interp.Fonts = info.Fonts
	interp.XObjects = info.XObjects
	if err := interp.Run(info.Content); err != nil {
		return nil, fmt.Errorf("content stream: %w", err)
	}
	return col, nil
}

func (p *page) Chars() ([]Char, error) {
	col, err := p.run()
	if err != nil {
		return nil, err
	}
	return col.chars, nil
}

func (p *page) Lines() ([]Line, error) {
	col, err := p.run()
	if err != nil {
		return nil, err
	}
	return col.lines, nil
}

func (p *page) Rects() ([]Rect, error) {
	col, err := p.run()
	if err != nil {
		return nil, err
	}
	return col.rects, nil
}

func (p *page) Curves() ([]Curve, error) {
	col, err := p.run()
	if err != nil {
		return nil, err
	}
	return col.curves, nil
}

func (p *page) Objects() (Objects, error) {
	col, err := p.run()
	if err != nil {
		return Objects{}, err
	}
	return Objects{
		Chars:  col.chars,
		Lines:  col.lines,
		Rects:  col.rects,
		Curves: col.curves,
	}, nil
}

// charsJoinedText is a tiny helper used by tests to assemble a Chars
// slice back into a string ignoring positioning. It is NOT a real
// text-extraction algorithm — Page.ExtractText (Phase 1.3.B) will
// sort by Y/X with line breaks; this is just for the smoke tests
// here in 1.3.A that need to assert "this page has the right glyphs
// in roughly the right order".
func charsJoinedText(chars []Char) string {
	var sb strings.Builder
	for _, c := range chars {
		sb.WriteString(c.Text)
	}
	return sb.String()
}
