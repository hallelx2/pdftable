// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdftable

// finder.go is the Go port of pdfplumber/table.py's TableFinder. The
// algorithm runs in four stages, each implemented as a pure function
// below so it can be unit-tested without spinning up a Page:
//
//  1. getEdges        — derive Lines/Rects/Curves from page primitives,
//                       apply prefilter, merge (snap + join), apply the
//                       post-merge min-length filter. Vertical edges
//                       from the "vertical_strategy" go in alongside the
//                       horizontal edges from the "horizontal_strategy".
//                       Implemented as Page.findTableEdges in page.go.
//  2. edgesToIntersections — pair every vertical edge with every
//                            horizontal edge and record the (x, y)
//                            intersection points whose perpendicular
//                            distance is within intersectionTolerance.
//  3. intersectionsToCells — for each intersection, walk down and right
//                            looking for the smallest closed rectangle
//                            whose four corners are all intersections
//                            joined by edges. Each found rectangle is
//                            one cell.
//  4. cellsToTables   — group cells that share at least one corner into
//                       the same table. Tables sorted top-to-bottom-
//                       then-left-to-right.
//
// Coordinate system note: pdfplumber operates in image space (Y growing
// DOWN). pdftable uses PDF user space (Y growing UP). The intersection
// algorithm is invariant under that flip — "below" in image space is
// "below" in user space if we substitute "lower Y" — but the wording in
// pdfplumber's source talks about "directly below and directly right".
// We keep the same algorithm, sorted by (Y descending, X ascending) so
// "below" means smaller Y (visually lower) and "right" means larger X.
// pdfplumber's points list is sorted ascending in image-space Y (so the
// FIRST point is the visually-topmost), which maps in user space to
// DESCENDING Y. The intersection logic uses point equality on (x, y)
// so the sort order only affects iteration order and the resulting
// cell list is the same.

import (
	"fmt"
	"sort"

	"github.com/hallelx2/pdftable/internal/layout"
)

// Intersection records one crossing point: an (x, y) tuple plus the
// vertical and horizontal edges that meet there. We need the edge sets
// (not just the count) because the cell-finder asks "does the same
// edge connect points p1 and p2?" — checking that two points lie on a
// shared edge is how the algorithm distinguishes "two intersections on
// the same ruler" from "two intersections on parallel rulers that
// happen to align".
//
// Field naming follows pdfplumber's intersections dict-of-dicts shape:
// the X/Y are the keys, V/H are the value lists. We keep them as slice
// fields so the struct is value-comparable on (X, Y) alone.
type Intersection struct {
	X, Y float64
	V    []layout.Edge // vertical edges passing through (X, Y)
	H    []layout.Edge // horizontal edges passing through (X, Y)
}

// TableBox is one detected table, expressed as a bbox plus a 2-D grid
// of cell bboxes. Rows are visually top-to-bottom; columns are left-to-
// right. CellsGrid[i][j] gives the bbox of the cell at row i, column j;
// missing cells (rectangular gaps in the grid) are reported as the
// zero BBox, NOT removed — callers can detect "this cell was missing"
// by checking IsZero on the entry.
//
// This is the geometry-only intermediate between FindTables and
// ExtractTables: FindTables returns one of these per detected table;
// ExtractTables then runs text-extraction per cell and wraps the
// result in a Table.
type TableBox struct {
	// BBox is the union of every cell's bbox.
	BBox BBox

	// Rows is the row count.
	Rows int

	// Cols is the column count.
	Cols int

	// CellsGrid is the per-cell bbox aligned to Rows × Cols. The
	// entry at [i][j] is the bbox of the cell at visual row i (0 is
	// topmost) and column j (0 is leftmost). Empty cells are the zero
	// BBox.
	CellsGrid [][]BBox
}

// Cells returns the cell bboxes flattened into reading order
// (left-to-right, top-to-bottom). Zero-bbox entries (holes in the
// grid) are skipped. Convenience helper for callers that want a single
// iterable.
func (t TableBox) Cells() []BBox {
	out := make([]BBox, 0, t.Rows*t.Cols)
	for _, row := range t.CellsGrid {
		for _, c := range row {
			if !c.IsZero() {
				out = append(out, c)
			}
		}
	}
	return out
}

// TableFinder is the geometry-only result of running the cells-from-
// edges pipeline on a page. It exposes the intermediate stages
// (edges, intersections, raw cells) alongside the assembled TableBox
// list so callers building debugging tools or custom text-extraction
// can see exactly what the pipeline produced.
//
// Pdfplumber bundles the page reference inside its TableFinder and
// exposes Table objects with an .extract() method; we keep the
// finder a pure value (no Page pointer) and let callers either grab
// the assembled Tables from Page.ExtractTables or compose their own
// text-fill loop using the public Cells and CellsGrid.
type TableFinder struct {
	// Edges is the merged, length-filtered edge list used as the
	// input to the intersection scan. Useful for debugging "why
	// didn't this rule get picked up" issues.
	Edges []layout.Edge

	// Intersections is the full set of edge crossings, keyed by
	// (X, Y). The order is deterministic — sorted by Y descending,
	// then X ascending — so callers can rely on iteration order.
	Intersections []Intersection

	// Cells is the raw list of detected cell bboxes BEFORE grouping
	// into tables. Each is a single rectangle whose four corners are
	// intersections joined by shared edges.
	Cells []BBox

	// Tables is the final list of detected tables. Each carries a
	// bbox plus a CellsGrid aligned to row/column order. Tables are
	// sorted top-to-bottom-then-left-to-right by their topmost cell.
	Tables []TableBox
}

// edgesToIntersections is the Go port of pdfplumber's
// edges_to_intersections. Given a slice of merged edges, return the
// list of crossing points where a vertical edge meets a horizontal
// edge within the supplied perpendicular tolerance.
//
// The algorithm:
//   - Split edges by orientation.
//   - Sort vertical edges by (X, smallest Y); sort horizontal edges
//     by (Y descending so top-to-bottom in user space, X).
//   - For each (v, h) pair, test whether the vertical edge's Y span
//     covers h.Y (within yTol) AND h's X span covers v.X (within xTol).
//     If yes, register (v.X, h.Y) as an intersection.
//
// We deduplicate intersection points on (X, Y) equality — if multiple
// edge pairs land on the same exact point, the V and H slices
// accumulate all participating edges. This matches pdfplumber's
// behaviour: its dict-keyed-on-vertex collapses repeats.
//
// xTol and yTol default to 0 if zero; the caller (TableSettings)
// already substitutes the pdfplumber default (3) before reaching this
// function.
func edgesToIntersections(edges []layout.Edge, xTol, yTol float64) []Intersection {
	if len(edges) == 0 {
		return nil
	}

	vEdges := layout.FilterEdgesByOrientation(edges, layout.Vertical)
	hEdges := layout.FilterEdgesByOrientation(edges, layout.Horizontal)

	// Sort: pdfplumber sorts v by (x0, top) and h by (top, x0). In
	// PDF user space "top" means LARGER Y. We sort v by (X0 asc,
	// Y0 asc) and h by (Y0 desc, X0 asc) so iteration order matches
	// the visual top-to-bottom traversal pdfplumber uses.
	sort.SliceStable(vEdges, func(i, j int) bool {
		if vEdges[i].X0 != vEdges[j].X0 {
			return vEdges[i].X0 < vEdges[j].X0
		}
		return vEdges[i].Y0 < vEdges[j].Y0
	})
	sort.SliceStable(hEdges, func(i, j int) bool {
		if hEdges[i].Y0 != hEdges[j].Y0 {
			return hEdges[i].Y0 > hEdges[j].Y0
		}
		return hEdges[i].X0 < hEdges[j].X0
	})

	// Key on (X, Y) using two floats — float64 comparisons are
	// exact-equality after snap_edges has unified positions onto
	// cluster means, so a struct key works without epsilon games.
	type key struct {
		x, y float64
	}
	indexByKey := make(map[key]int)
	var out []Intersection

	for _, v := range vEdges {
		for _, h := range hEdges {
			// pdfplumber's test (translated from image-space "top" /
			// "bottom" to PDF user-space Y0 / Y1):
			//
			//   image space: v.top <= h.top + yTol  AND
			//                v.bottom >= h.top - yTol
			//
			// In image space "top" is the smaller Y (visually higher)
			// and "bottom" is the larger Y. In PDF user space the
			// orientation flips: "visually higher" means LARGER Y.
			//
			// The two conditions in image space say: h.top is between
			// v.top and v.bottom (i.e. v's Y range covers h's Y). In
			// PDF user space the same constraint is:
			//
			//   v.Y0 <= h.Y0 + yTol   AND   v.Y1 >= h.Y0 - yTol
			//
			// — the horizontal edge's Y (which equals h.Y0 == h.Y1)
			// must lie within the vertical edge's [Y0, Y1] span,
			// with yTol slack on both ends.
			if v.Y0 > h.Y0+yTol {
				continue
			}
			if v.Y1 < h.Y0-yTol {
				continue
			}
			// h must cover v.X0 with xTol slack.
			if v.X0 < h.X0-xTol {
				continue
			}
			if v.X0 > h.X1+xTol {
				continue
			}
			k := key{x: v.X0, y: h.Y0}
			if idx, ok := indexByKey[k]; ok {
				out[idx].V = append(out[idx].V, v)
				out[idx].H = append(out[idx].H, h)
			} else {
				indexByKey[k] = len(out)
				out = append(out, Intersection{
					X: v.X0,
					Y: h.Y0,
					V: []layout.Edge{v},
					H: []layout.Edge{h},
				})
			}
		}
	}

	// Deterministic order: pdfplumber sorts the keys ascending in
	// (image-space) top, then x0 — which in user space is Y
	// DESCENDING (visually top first), then X ascending.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Y != out[j].Y {
			return out[i].Y > out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}

// edgeSharedKey is the equality test used to decide whether two
// intersection lists share an edge. We hash edges by their full
// geometric tuple so that two intersections that lie on the SAME
// merged edge (post snap+join, so identical X/Y) share the key.
type edgeSharedKey struct {
	x0, y0, x1, y1 float64
	o              layout.Orientation
}

func edgeKey(e layout.Edge) edgeSharedKey {
	return edgeSharedKey{x0: e.X0, y0: e.Y0, x1: e.X1, y1: e.Y1, o: e.Orientation}
}

// edgeConnects reports whether p1 and p2 share an edge — i.e. lie on
// the same merged ruler. This is the predicate pdfplumber uses to
// distinguish "two points on the same line" from "two points on
// parallel lines that happen to align".
//
// p1 and p2 must share an axis (same X for vertical-shared, same Y
// for horizontal-shared); the function returns false otherwise.
func edgeConnects(p1, p2 Intersection) bool {
	if p1.X == p2.X {
		// Look for a vertical edge present in both intersections'
		// V slices.
		seen := make(map[edgeSharedKey]struct{}, len(p1.V))
		for _, e := range p1.V {
			seen[edgeKey(e)] = struct{}{}
		}
		for _, e := range p2.V {
			if _, ok := seen[edgeKey(e)]; ok {
				return true
			}
		}
	}
	if p1.Y == p2.Y {
		seen := make(map[edgeSharedKey]struct{}, len(p1.H))
		for _, e := range p1.H {
			seen[edgeKey(e)] = struct{}{}
		}
		for _, e := range p2.H {
			if _, ok := seen[edgeKey(e)]; ok {
				return true
			}
		}
	}
	return false
}

// intersectionsToCells is the Go port of pdfplumber's
// intersections_to_cells. Given a sorted-by-(Y desc, X asc) list of
// intersections, return the smallest closed rectangle anchored at
// each intersection — that's one cell.
//
// The algorithm at each point pt:
//   - Find all `below` points (same X as pt, lower Y in user space).
//   - Find all `right` points  (same Y as pt, larger X).
//   - For every below_pt with which pt shares a vertical edge, and
//     every right_pt with which pt shares a horizontal edge, check
//     whether the diagonal corner (right_pt.X, below_pt.Y) is also an
//     intersection AND shares the necessary edges to close the
//     rectangle. If yes, that's the smallest cell — return it.
//
// "Smallest" comes from the sort order: `below` and `right` slices
// preserve the sorted suffix of the intersections list, so the
// nearest points are tried first.
func intersectionsToCells(intersections []Intersection) []BBox {
	if len(intersections) == 0 {
		return nil
	}

	// Index by (X, Y) for fast "is this corner an intersection" lookup.
	type key struct {
		x, y float64
	}
	idxByKey := make(map[key]int, len(intersections))
	for i, p := range intersections {
		idxByKey[key{x: p.X, y: p.Y}] = i
	}

	// In user space the intersections list is sorted by Y descending
	// (visually top first) then X ascending. "directly below pt"
	// means same X with smaller Y; "directly right" means same Y
	// with larger X. Both are in the SUFFIX of the sorted list,
	// matching pdfplumber's "rest = points[i+1:]" walk.
	var cells []BBox
	for i, pt := range intersections {
		if i == len(intersections)-1 {
			break
		}
		rest := intersections[i+1:]

		// below: same X, smaller Y (already true for the suffix
		// because the list is Y-descending and X-ascending; same X
		// entries that follow pt all have smaller Y).
		var below []Intersection
		var right []Intersection
		for _, q := range rest {
			if q.X == pt.X && q.Y < pt.Y {
				below = append(below, q)
			}
			if q.Y == pt.Y && q.X > pt.X {
				right = append(right, q)
			}
		}

		found := false
		for _, bp := range below {
			if !edgeConnects(pt, bp) {
				continue
			}
			for _, rp := range right {
				if !edgeConnects(pt, rp) {
					continue
				}
				cornerKey := key{x: rp.X, y: bp.Y}
				cornerIdx, ok := idxByKey[cornerKey]
				if !ok {
					continue
				}
				corner := intersections[cornerIdx]
				if !edgeConnects(corner, rp) {
					continue
				}
				if !edgeConnects(corner, bp) {
					continue
				}
				// Cell corners in PDF user space:
				//   pt (top-left)        : (pt.X,  pt.Y)
				//   rp (top-right)       : (rp.X,  pt.Y)
				//   bp (bottom-left)     : (pt.X,  bp.Y)
				//   corner (bottom-right): (rp.X,  bp.Y)
				// Cell bbox: x0 = pt.X, x1 = rp.X, y0 = bp.Y, y1 = pt.Y.
				cells = append(cells, NewBBox(pt.X, bp.Y, rp.X, pt.Y))
				found = true
				break
			}
			if found {
				break
			}
		}
	}
	return cells
}

// cellsToTables is the Go port of pdfplumber's cells_to_tables. Given
// the raw cells from intersectionsToCells, group cells that share at
// least one corner into the same table. Standalone cells (those that
// don't touch any other cell on a corner) are dropped — a "table" of
// one cell is almost always a decorative box, not real tabular data.
//
// The implementation:
//   - Initialise a current table with the first remaining cell.
//   - In a pass over the remaining cells, append every cell that shares
//     at least one corner with the current table; remove appended
//     cells from the remaining list.
//   - Repeat until no more cells get appended in a pass; close the
//     current table and start a new one with the next remaining cell.
//   - Drop tables with fewer than 2 cells.
//
// Returns each table as a 1-D slice of its constituent cells; the
// caller (assembleTableBox) then projects the cells into the row /
// column grid.
func cellsToTables(cells []BBox) [][]BBox {
	if len(cells) == 0 {
		return nil
	}
	remaining := make([]BBox, len(cells))
	copy(remaining, cells)

	type corner struct {
		x, y float64
	}
	bboxCorners := func(b BBox) [4]corner {
		return [4]corner{
			{b.X0, b.Y1}, // top-left
			{b.X0, b.Y0}, // bottom-left
			{b.X1, b.Y1}, // top-right
			{b.X1, b.Y0}, // bottom-right
		}
	}

	var tables [][]BBox
	currentCells := make([]BBox, 0)
	currentCorners := make(map[corner]struct{})

	for len(remaining) > 0 {
		initialCount := len(currentCells)
		// One pass over remaining; collect newly assigned indices to
		// remove after the pass so we don't disturb the slice during
		// iteration.
		assigned := make([]int, 0, len(remaining))
		for i, cell := range remaining {
			cc := bboxCorners(cell)
			if len(currentCells) == 0 {
				for _, c := range cc {
					currentCorners[c] = struct{}{}
				}
				currentCells = append(currentCells, cell)
				assigned = append(assigned, i)
				continue
			}
			cornerCount := 0
			for _, c := range cc {
				if _, ok := currentCorners[c]; ok {
					cornerCount++
				}
			}
			if cornerCount > 0 {
				for _, c := range cc {
					currentCorners[c] = struct{}{}
				}
				currentCells = append(currentCells, cell)
				assigned = append(assigned, i)
			}
		}
		// Apply removals in reverse so indices stay valid.
		sort.Sort(sort.Reverse(sort.IntSlice(assigned)))
		for _, idx := range assigned {
			remaining = append(remaining[:idx], remaining[idx+1:]...)
		}

		if len(currentCells) == initialCount {
			// Nothing was added this pass — close the table and start
			// a fresh one with the next remaining cell.
			tables = append(tables, currentCells)
			currentCells = make([]BBox, 0)
			currentCorners = make(map[corner]struct{})
		}
	}
	if len(currentCells) > 0 {
		tables = append(tables, currentCells)
	}

	// Sort tables visually top-to-bottom, then left-to-right. We key
	// each table on its topmost-leftmost cell — pdfplumber uses
	// min((top, x0)) (image space); in user space that's the cell
	// with the LARGEST Y1 (visual top), tie-broken by smallest X0.
	sort.SliceStable(tables, func(i, j int) bool {
		ti := pickTopLeft(tables[i])
		tj := pickTopLeft(tables[j])
		if ti.Y1 != tj.Y1 {
			return ti.Y1 > tj.Y1
		}
		return ti.X0 < tj.X0
	})

	// Drop standalone-cell tables (pdfplumber: `len(t) > 1`).
	filtered := tables[:0]
	for _, t := range tables {
		if len(t) > 1 {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// pickTopLeft returns the cell that's visually topmost (largest Y1)
// and leftmost on ties (smallest X0).
func pickTopLeft(cells []BBox) BBox {
	best := cells[0]
	for _, c := range cells[1:] {
		if c.Y1 > best.Y1 || (c.Y1 == best.Y1 && c.X0 < best.X0) {
			best = c
		}
	}
	return best
}

// assembleTableBox projects a flat list of cells into a 2-D
// row/column grid. The algorithm collects the unique X0 values
// (column lefts) and Y1 values (row tops) across all cells, sorts
// them, and indexes each cell by (row from Y1, column from X0).
//
// Holes in the grid (a cell that should be at row i, col j but
// wasn't detected) remain as zero BBox entries — the caller can
// detect them with IsZero. We don't try to "fill" them by inferring
// boundaries; pdfplumber doesn't either.
func assembleTableBox(cells []BBox) TableBox {
	if len(cells) == 0 {
		return TableBox{}
	}

	// Collect unique row/column anchor positions. Row 0 is visually
	// topmost — largest Y1; column 0 is leftmost — smallest X0.
	xs := make(map[float64]struct{})
	ys := make(map[float64]struct{})
	for _, c := range cells {
		xs[c.X0] = struct{}{}
		ys[c.Y1] = struct{}{}
	}
	xList := make([]float64, 0, len(xs))
	for x := range xs {
		xList = append(xList, x)
	}
	yList := make([]float64, 0, len(ys))
	for y := range ys {
		yList = append(yList, y)
	}
	sort.Float64s(xList)
	sort.Slice(yList, func(i, j int) bool { return yList[i] > yList[j] })

	xIndex := make(map[float64]int, len(xList))
	for i, x := range xList {
		xIndex[x] = i
	}
	yIndex := make(map[float64]int, len(yList))
	for i, y := range yList {
		yIndex[y] = i
	}

	rows := len(yList)
	cols := len(xList)
	grid := make([][]BBox, rows)
	for i := range grid {
		grid[i] = make([]BBox, cols)
	}

	// Union bbox.
	bbox := cells[0]
	for _, c := range cells {
		ri, ok1 := yIndex[c.Y1]
		ci, ok2 := xIndex[c.X0]
		if ok1 && ok2 {
			grid[ri][ci] = c
		}
		bbox = bbox.Union(c)
	}

	return TableBox{
		BBox:      bbox,
		Rows:      rows,
		Cols:      cols,
		CellsGrid: grid,
	}
}

// runTableFinder is the geometry-only pipeline: given the page's
// edges (already merged + length-filtered by Page.findTableEdges) and
// the intersection / settings tolerances, build a TableFinder.
//
// This is the seam between page.go (which knows how to enumerate the
// page primitives) and the algorithms in this file. Splitting it out
// keeps the algorithm tests in table_test.go fast: they construct
// edges in-memory and call this function directly without ever
// opening a PDF.
func runTableFinder(edges []layout.Edge, xTol, yTol float64) TableFinder {
	intersections := edgesToIntersections(edges, xTol, yTol)
	cells := intersectionsToCells(intersections)
	groups := cellsToTables(cells)
	tables := make([]TableBox, 0, len(groups))
	for _, g := range groups {
		tables = append(tables, assembleTableBox(g))
	}
	return TableFinder{
		Edges:         edges,
		Intersections: intersections,
		Cells:         cells,
		Tables:        tables,
	}
}

// ensureSupportedStrategies returns an error if either strategy is
// "text" or "explicit" — those are deferred to Phase 1.3.D (v0.3.0).
// Returning a clear ErrUnsupported keeps callers from silently getting
// empty results when they ask for a strategy we don't implement yet.
func ensureSupportedStrategies(s TableSettings) error {
	for _, pair := range []struct {
		axis     string
		strategy TableStrategy
	}{
		{"vertical", s.VerticalStrategy},
		{"horizontal", s.HorizontalStrategy},
	} {
		switch pair.strategy {
		case StrategyLines, StrategyLinesStrict:
			// ok
		case StrategyText:
			return fmt.Errorf("%w: %s_strategy=%q (Phase 1.3.D)", ErrUnsupported, pair.axis, pair.strategy)
		case StrategyExplicit:
			return fmt.Errorf("%w: %s_strategy=%q (Phase 1.3.D)", ErrUnsupported, pair.axis, pair.strategy)
		default:
			return fmt.Errorf("%w: unknown %s_strategy %q", ErrUnsupported, pair.axis, pair.strategy)
		}
	}
	return nil
}
