// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdftable

import "math"

// BBox is the canonical four-tuple bounding-box helper that the layout
// algorithms (clustering, word grouping, text extraction) operate on.
// Field naming follows the Char/Line/Rect convention used throughout
// the package: x0,y0 is the lower-left corner and x1,y1 is the upper-
// right corner in PDF user space (origin at bottom-left, Y growing up).
//
// We expose BBox as a value type — small, stack-allocatable, trivially
// copyable. Algorithms that need to pass a bbox around without poking
// at the larger Char/Rect/Line wrappers can construct one with NewBBox
// or pull one out with the BBoxOf helpers below.
//
// The Go API intentionally chooses (X0,Y0,X1,Y1) over pdfplumber's
// dict-of-strings ({"x0","top","x1","bottom"}). The two flavours differ
// because pdfplumber operates in image space (Y growing down, "top" =
// small Y, "bottom" = large Y) and we operate in PDF user space (Y
// growing up). Comments call out the mapping wherever it matters.
type BBox struct {
	X0, Y0, X1, Y1 float64
}

// NewBBox builds a BBox and normalises it so X0<=X1 and Y0<=Y1.
// Algorithms downstream rely on the normal form, so we never let an
// inverted bbox leak past this constructor.
func NewBBox(x0, y0, x1, y1 float64) BBox {
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	return BBox{X0: x0, Y0: y0, X1: x1, Y1: y1}
}

// Width returns the bbox's horizontal extent.
func (b BBox) Width() float64 { return b.X1 - b.X0 }

// Height returns the bbox's vertical extent.
func (b BBox) Height() float64 { return b.Y1 - b.Y0 }

// Area returns Width * Height.
func (b BBox) Area() float64 { return b.Width() * b.Height() }

// IsZero reports whether the bbox is the zero value (all four fields
// equal to zero). Useful for "did I forget to populate this" checks.
func (b BBox) IsZero() bool { return b == BBox{} }

// Union returns the smallest bbox enclosing both b and other.
//
// We DON'T treat a zero-value BBox as "empty" here — a caller that
// passes BBox{} to Union genuinely means "enclose the origin point".
// Use MergeBBoxes when you have a slice and want it to be a no-op on
// the empty slice.
func (b BBox) Union(other BBox) BBox {
	return BBox{
		X0: math.Min(b.X0, other.X0),
		Y0: math.Min(b.Y0, other.Y0),
		X1: math.Max(b.X1, other.X1),
		Y1: math.Max(b.Y1, other.Y1),
	}
}

// Intersect returns the overlapping rectangle of b and other, and a
// boolean reporting whether the intersection has non-empty area (i.e.
// the two bboxes actually overlap). This mirrors pdfplumber's
// get_bbox_overlap, which returns None when the boxes don't touch and
// the overlapping bbox otherwise.
//
// We treat touching-but-not-overlapping (shared edge, zero area) as
// non-overlap, matching pdfplumber's `o_height + o_width > 0` check —
// a single-line ruler that grazes a word's bbox should not be reported
// as "intersecting" the word.
func (b BBox) Intersect(other BBox) (BBox, bool) {
	oLeft := math.Max(b.X0, other.X0)
	oRight := math.Min(b.X1, other.X1)
	oBottom := math.Max(b.Y0, other.Y0)
	oTop := math.Min(b.Y1, other.Y1)

	w := oRight - oLeft
	h := oTop - oBottom
	// pdfplumber requires width>=0, height>=0, AND width+height>0
	// (so zero-area overlaps don't count). Matching that exactly.
	if w < 0 || h < 0 || w+h <= 0 {
		return BBox{}, false
	}
	return BBox{X0: oLeft, Y0: oBottom, X1: oRight, Y1: oTop}, true
}

// Contains reports whether b fully encloses other. Edges are
// considered inside (>= on the low side, <= on the high side), so a
// bbox contains itself.
func (b BBox) Contains(other BBox) bool {
	return other.X0 >= b.X0 &&
		other.Y0 >= b.Y0 &&
		other.X1 <= b.X1 &&
		other.Y1 <= b.Y1
}

// ContainsPoint reports whether (x,y) lies inside b (inclusive on
// edges).
func (b BBox) ContainsPoint(x, y float64) bool {
	return x >= b.X0 && x <= b.X1 && y >= b.Y0 && y <= b.Y1
}

// Snap rounds each of b's four coordinates to the nearest multiple of
// step. Used by layout-analysis code to coalesce near-equal positions
// (e.g. ruling lines drawn at 99.9, 100.0, 100.1) before clustering.
// A step of 0 returns the original bbox unchanged.
func (b BBox) Snap(step float64) BBox {
	if step == 0 {
		return b
	}
	return BBox{
		X0: math.Round(b.X0/step) * step,
		Y0: math.Round(b.Y0/step) * step,
		X1: math.Round(b.X1/step) * step,
		Y1: math.Round(b.Y1/step) * step,
	}
}

// MergeBBoxes returns the smallest bbox enclosing every input. Empty
// input returns a zero BBox. This mirrors pdfplumber's merge_bboxes
// and objects_to_bbox helpers — the typical caller has a slice of
// Chars and wants the combined bounding box for the resulting Word.
func MergeBBoxes(bboxes []BBox) BBox {
	if len(bboxes) == 0 {
		return BBox{}
	}
	out := bboxes[0]
	for _, bb := range bboxes[1:] {
		out = out.Union(bb)
	}
	return out
}

// BBoxOfChar returns the bounding box of a Char.
func BBoxOfChar(c Char) BBox {
	return BBox{X0: c.X0, Y0: c.Y0, X1: c.X1, Y1: c.Y1}
}

// BBoxOfChars returns the smallest bbox enclosing every char in cs.
// Returns the zero BBox for an empty slice.
func BBoxOfChars(cs []Char) BBox {
	if len(cs) == 0 {
		return BBox{}
	}
	out := BBoxOfChar(cs[0])
	for _, c := range cs[1:] {
		out = out.Union(BBoxOfChar(c))
	}
	return out
}
