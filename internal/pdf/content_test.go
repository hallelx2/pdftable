// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdf

import (
	"math"
	"testing"
)

// captureSink is a Sink that records every event for assertions.
type captureSink struct {
	chars []CharEvent
	paths []PathEvent
}

func (s *captureSink) EmitChar(ev CharEvent) { s.chars = append(s.chars, ev) }
func (s *captureSink) EmitPath(ev PathEvent) { s.paths = append(s.paths, ev) }

// TestInterpreterTextMatrix feeds a hand-crafted content stream that
// declares a font, sets the text matrix to a known position, and
// draws a couple of glyphs. We assert that the chars come out at the
// expected user-space positions — this is the load-bearing piece of
// the whole interpreter, so we test it directly with no PDF wrapper.
func TestInterpreterTextMatrix(t *testing.T) {
	// Build a fake font: simple WinAnsi, 500-unit widths, 0 descent.
	font := &Font{
		BaseFont:            "TestFont",
		IsSimple:            true,
		cid2unicodeEncoding: EncodingByName("WinAnsiEncoding"),
		Widths:              map[uint16]float64{},
		DefaultWidth:        500, // half em.
		Ascent:              700,
		Descent:             0,
	}
	// Make widths for 'H' and 'i'.
	font.Widths[uint16('H')] = 600
	font.Widths[uint16('i')] = 300

	sink := &captureSink{}
	it := NewInterpreter(IdentityMatrix, sink)
	it.Fonts = map[string]*Font{"F1": font}

	// Stream: select font, position at (100, 200), draw "Hi".
	stream := []byte(`BT
/F1 10 Tf
100 200 Td
(Hi) Tj
ET
`)
	if err := it.Run(stream); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sink.chars) != 2 {
		t.Fatalf("got %d chars, want 2", len(sink.chars))
	}
	if sink.chars[0].Text != "H" || sink.chars[1].Text != "i" {
		t.Errorf("chars = %q, %q; want H, i", sink.chars[0].Text, sink.chars[1].Text)
	}
	// 'H' should start at X0 ≈ 100, Y0 ≈ 200.
	if !approxEq(sink.chars[0].X0, 100) {
		t.Errorf("H X0 = %v, want 100", sink.chars[0].X0)
	}
	if !approxEq(sink.chars[0].Y0, 200) {
		t.Errorf("H Y0 = %v, want 200", sink.chars[0].Y0)
	}
	// FontSize is the Tf-declared size at identity CTM.
	if sink.chars[0].FontSize != 10 {
		t.Errorf("H FontSize = %v, want 10", sink.chars[0].FontSize)
	}
	// 'i' should be advanced by H's width = 600 * 10 / 1000 = 6 pt.
	if !approxEq(sink.chars[1].X0, 106) {
		t.Errorf("i X0 = %v, want 106", sink.chars[1].X0)
	}
	if sink.chars[0].FontName != "TestFont" {
		t.Errorf("FontName = %q, want TestFont", sink.chars[0].FontName)
	}
	if !sink.chars[0].Upright {
		t.Errorf("Upright = false, want true")
	}
}

// TestInterpreterTJArray exercises the TJ operator (positioned
// glyphs with embedded kerning numbers). The numbers shift the
// text-space X by -n * fontSize / 1000.
func TestInterpreterTJArray(t *testing.T) {
	font := &Font{
		BaseFont:            "TestFont",
		IsSimple:            true,
		cid2unicodeEncoding: EncodingByName("WinAnsiEncoding"),
		Widths:              map[uint16]float64{},
		DefaultWidth:        500,
	}
	font.Widths[uint16('A')] = 500
	font.Widths[uint16('B')] = 500

	sink := &captureSink{}
	it := NewInterpreter(IdentityMatrix, sink)
	it.Fonts = map[string]*Font{"F1": font}

	// "A" then -100 kerning (i.e. shift LEFT by -(-100)/1000 * 10 = 1 pt
	// since +adj = -elem.Number * dxscale; negative number widens gap)
	// then "B".
	stream := []byte(`BT
/F1 10 Tf
0 0 Td
[(A) -100 (B)] TJ
ET
`)
	if err := it.Run(stream); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sink.chars) != 2 {
		t.Fatalf("got %d chars, want 2", len(sink.chars))
	}
	// A width = 5 pt. B's natural position would be 5; TJ -100 widens
	// the gap (kerning convention) so B sits a bit further right.
	gap := sink.chars[1].X0 - sink.chars[0].X1
	if gap < 0.5 || gap > 1.5 {
		t.Errorf("kerning gap = %v, expected ~1 pt", gap)
	}
}

// TestInterpreterPath checks that an `re` rectangle survives through
// the interpreter as a single PathEvent with the right segments.
func TestInterpreterPath(t *testing.T) {
	sink := &captureSink{}
	it := NewInterpreter(IdentityMatrix, sink)
	stream := []byte(`50 60 100 200 re
S
`)
	if err := it.Run(stream); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sink.paths) != 1 {
		t.Fatalf("got %d paths, want 1", len(sink.paths))
	}
	p := sink.paths[0]
	if !p.Stroke || p.Fill {
		t.Errorf("paint flags = stroke=%v fill=%v, want stroke=true fill=false", p.Stroke, p.Fill)
	}
	if len(p.Segments) != 1 || p.Segments[0].Op != "re" {
		t.Fatalf("expected single re segment, got %v", p.Segments)
	}
	if !approxEq(p.Segments[0].X, 50) || !approxEq(p.Segments[0].Y, 60) ||
		!approxEq(p.Segments[0].W, 100) || !approxEq(p.Segments[0].H, 200) {
		t.Errorf("re params = (%v,%v,%v,%v), want (50,60,100,200)",
			p.Segments[0].X, p.Segments[0].Y, p.Segments[0].W, p.Segments[0].H)
	}
}

// TestInterpreterPathLineSegments confirms `m`/`l` paths come through
// as a sequence of segments with the right ops.
func TestInterpreterPathLineSegments(t *testing.T) {
	sink := &captureSink{}
	it := NewInterpreter(IdentityMatrix, sink)
	stream := []byte(`10 20 m
30 40 l
S
`)
	if err := it.Run(stream); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sink.paths) != 1 {
		t.Fatalf("got %d paths, want 1", len(sink.paths))
	}
	p := sink.paths[0]
	if len(p.Segments) != 2 {
		t.Fatalf("got %d segments, want 2", len(p.Segments))
	}
	if p.Segments[0].Op != "m" || p.Segments[1].Op != "l" {
		t.Errorf("segments ops = %s %s, want m l", p.Segments[0].Op, p.Segments[1].Op)
	}
}

// TestMatrixMultiply checks the matrix math we use everywhere.
//
// Our Mult follows pdfminer's mult_matrix convention: `Mult(m2, m1)`
// returns the matrix M such that applying M to a point is equivalent
// to applying m2 to the point first, then applying m1 to the result.
// (PDF concatenates transforms via post-multiplication in this
// row-vector convention.)
//
// We build translate (offset (10,20)) and scale (×2 in both axes),
// then Mult(translate, scale). Per convention: apply translate first,
// then scale → (x*2+e*2, y*2+f*2). For (5,7): translate gives (15,27);
// scale gives (30,54).
func TestMatrixMultiply(t *testing.T) {
	scale := Matrix{2, 0, 0, 2, 0, 0}
	translate := Matrix{1, 0, 0, 1, 10, 20}

	m := Mult(translate, scale)
	x, y := ApplyPoint(m, 5, 7)
	if !approxEq(x, 30) || !approxEq(y, 54) {
		t.Errorf("ApplyPoint = (%v,%v), want (30,54)", x, y)
	}

	// Reverse order: scale first then translate.
	// Apply scale (×2) then translate (+10,+20). For (5,7): scale gives
	// (10,14); translate gives (20,34).
	m2 := Mult(scale, translate)
	x2, y2 := ApplyPoint(m2, 5, 7)
	if !approxEq(x2, 20) || !approxEq(y2, 34) {
		t.Errorf("reversed ApplyPoint = (%v,%v), want (20,34)", x2, y2)
	}
}

func TestTranslateMatrix(t *testing.T) {
	m := Translate(IdentityMatrix, 100, 200)
	x, y := ApplyPoint(m, 0, 0)
	if !approxEq(x, 100) || !approxEq(y, 200) {
		t.Errorf("translated origin = (%v,%v), want (100,200)", x, y)
	}
}

// TestGraphicsStateStack confirms q/Q save and restore the CTM.
func TestGraphicsStateStack(t *testing.T) {
	sink := &captureSink{}
	it := NewInterpreter(IdentityMatrix, sink)
	stream := []byte(`q
2 0 0 2 0 0 cm
50 50 10 10 re
S
Q
0 0 5 5 re
S
`)
	if err := it.Run(stream); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sink.paths) != 2 {
		t.Fatalf("got %d paths, want 2", len(sink.paths))
	}
	// First rect was drawn under 2× scale CTM: 50→100, 10→20.
	r0 := sink.paths[0].Segments[0]
	if !approxEq(r0.X, 100) || !approxEq(r0.W, 20) {
		t.Errorf("scaled rect = (%v,W=%v), want (100, W=20)", r0.X, r0.W)
	}
	// Second rect was drawn after Q restored the identity CTM.
	r1 := sink.paths[1].Segments[0]
	if !approxEq(r1.X, 0) || !approxEq(r1.W, 5) {
		t.Errorf("restored-CTM rect = (%v,W=%v), want (0, W=5)", r1.X, r1.W)
	}
}

func approxEq(a, b float64) bool { return math.Abs(a-b) < 0.5 }
