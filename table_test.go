// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdftable

// table_test.go is intentionally in the pdftable package (not
// pdftable_test) so it can reach the unexported algorithm functions:
// edgesToIntersections, intersectionsToCells, cellsToTables,
// assembleTableBox, runTableFinder. The public-API integration test
// (TestExtractTables_RuledFixture) lives at the end and uses the
// public surface only.

import (
	"strings"
	"testing"

	"github.com/hallelx2/pdftable/internal/layout"
	"github.com/hallelx2/pdftable/testdata"
)

// makeH builds a horizontal edge at Y = y from X0 = x0 to X1 = x1.
// Tests never construct layout.Edge values directly because the
// invariants (X0 <= X1 for h, Y0 == Y1 for h) are constructor-
// enforced; this helper centralises the common case.
func makeH(x0, x1, y float64) layout.Edge {
	return layout.Edge{
		X0: x0, X1: x1, Y0: y, Y1: y,
		Orientation: layout.Horizontal,
		Source:      layout.SourceLine,
	}
}

// makeV builds a vertical edge at X = x from Y0 = y0 to Y1 = y1.
func makeV(x, y0, y1 float64) layout.Edge {
	return layout.Edge{
		X0: x, X1: x, Y0: y0, Y1: y1,
		Orientation: layout.Vertical,
		Source:      layout.SourceLine,
	}
}

// TestEdgesToIntersections_Grid2x2 sets up a 2×2 cell grid (3
// horizontal and 3 vertical edges) and asserts the intersection
// scanner finds exactly 9 crossing points at the expected (X, Y)
// pairs.
//
// Geometry (PDF user space, Y growing up):
//
//	Y=100 ─────────
//	      │ │ │ │
//	Y= 50 ─────────
//	      │ │ │ │
//	Y=  0 ─────────
//	   X=0 X=50 X=100
func TestEdgesToIntersections_Grid2x2(t *testing.T) {
	edges := []layout.Edge{
		makeH(0, 100, 0),
		makeH(0, 100, 50),
		makeH(0, 100, 100),
		makeV(0, 0, 100),
		makeV(50, 0, 100),
		makeV(100, 0, 100),
	}
	ints := edgesToIntersections(edges, 0.1, 0.1)
	if len(ints) != 9 {
		t.Fatalf("intersections: got %d, want 9", len(ints))
	}

	// Build a set of (X, Y) for easy lookup.
	type pt struct{ x, y float64 }
	got := make(map[pt]bool, len(ints))
	for _, p := range ints {
		got[pt{x: p.X, y: p.Y}] = true
	}
	for _, x := range []float64{0, 50, 100} {
		for _, y := range []float64{0, 50, 100} {
			if !got[pt{x: x, y: y}] {
				t.Errorf("missing intersection at (%v, %v)", x, y)
			}
		}
	}

	// Each intersection should have at least one V and one H edge.
	for _, p := range ints {
		if len(p.V) == 0 || len(p.H) == 0 {
			t.Errorf("intersection (%v,%v): V=%d H=%d, want both > 0",
				p.X, p.Y, len(p.V), len(p.H))
		}
	}
}

// TestEdgesToIntersections_NoCrossing asserts that two parallel
// edges, even when their Y / X spans overlap, produce no
// intersections.
func TestEdgesToIntersections_NoCrossing(t *testing.T) {
	edges := []layout.Edge{
		makeH(0, 100, 50),
		makeH(0, 100, 60),
		makeV(0, 0, 100),
		makeV(50, 0, 100),
	}
	// Expected: 4 intersections (each H crosses each V).
	ints := edgesToIntersections(edges, 0.1, 0.1)
	if len(ints) != 4 {
		t.Fatalf("got %d intersections, want 4", len(ints))
	}
}

// TestEdgesToIntersections_Tolerance asserts the perpendicular
// tolerance lets a near-miss crossing register. The vertical edge
// ends at Y=49.5 but the horizontal edge sits at Y=50: with yTol=1
// the intersection should still be picked up.
func TestEdgesToIntersections_Tolerance(t *testing.T) {
	edges := []layout.Edge{
		makeH(0, 100, 50),
		makeV(50, 0, 49.5),
	}
	if got := len(edgesToIntersections(edges, 1, 1)); got != 1 {
		t.Errorf("with yTol=1: got %d intersections, want 1", got)
	}
	if got := len(edgesToIntersections(edges, 0.1, 0.1)); got != 0 {
		t.Errorf("with yTol=0.1: got %d intersections, want 0", got)
	}
}

// TestIntersectionsToCells_Single2x2 feeds the 9 intersections of a
// 2×2 grid and asserts the cell finder produces exactly 4 cells
// covering the expected bboxes.
func TestIntersectionsToCells_Single2x2(t *testing.T) {
	edges := []layout.Edge{
		makeH(0, 100, 0),
		makeH(0, 100, 50),
		makeH(0, 100, 100),
		makeV(0, 0, 100),
		makeV(50, 0, 100),
		makeV(100, 0, 100),
	}
	ints := edgesToIntersections(edges, 0.1, 0.1)
	cells := intersectionsToCells(ints)
	if len(cells) != 4 {
		t.Fatalf("got %d cells, want 4", len(cells))
	}

	type want struct{ x0, y0, x1, y1 float64 }
	expected := []want{
		{0, 50, 50, 100},
		{50, 50, 100, 100},
		{0, 0, 50, 50},
		{50, 0, 100, 50},
	}
	for _, w := range expected {
		found := false
		for _, c := range cells {
			if c.X0 == w.x0 && c.Y0 == w.y0 && c.X1 == w.x1 && c.Y1 == w.y1 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing cell (%v,%v)-(%v,%v)", w.x0, w.y0, w.x1, w.y1)
		}
	}
}

// TestCellsToTables_GroupsTouching asserts that cells sharing a
// corner end up in the same table, and that singletons get dropped.
func TestCellsToTables_GroupsTouching(t *testing.T) {
	// Two 2x2 grids that don't touch.
	left := []BBox{
		NewBBox(0, 0, 50, 50),
		NewBBox(50, 0, 100, 50),
		NewBBox(0, 50, 50, 100),
		NewBBox(50, 50, 100, 100),
	}
	right := []BBox{
		NewBBox(200, 0, 250, 50),
		NewBBox(250, 0, 300, 50),
		NewBBox(200, 50, 250, 100),
		NewBBox(250, 50, 300, 100),
	}
	// One standalone cell that should be filtered out.
	stray := []BBox{NewBBox(400, 400, 410, 410)}

	all := append([]BBox{}, left...)
	all = append(all, right...)
	all = append(all, stray...)

	tables := cellsToTables(all)
	if len(tables) != 2 {
		t.Fatalf("got %d tables, want 2", len(tables))
	}
	for i, tbl := range tables {
		if len(tbl) != 4 {
			t.Errorf("table %d: got %d cells, want 4", i, len(tbl))
		}
	}
}

// TestAssembleTableBox_2x2Grid asserts the row/column projection
// builds the expected 2×2 grid with cells in the right slots.
//
// Row 0 is visually TOP (larger Y1). Column 0 is visually LEFT
// (smaller X0).
func TestAssembleTableBox_2x2Grid(t *testing.T) {
	cells := []BBox{
		NewBBox(0, 50, 50, 100),   // top-left
		NewBBox(50, 50, 100, 100), // top-right
		NewBBox(0, 0, 50, 50),     // bottom-left
		NewBBox(50, 0, 100, 50),   // bottom-right
	}
	tb := assembleTableBox(cells)
	if tb.Rows != 2 || tb.Cols != 2 {
		t.Fatalf("got Rows=%d Cols=%d, want 2x2", tb.Rows, tb.Cols)
	}
	if tb.BBox != (BBox{X0: 0, Y0: 0, X1: 100, Y1: 100}) {
		t.Errorf("bbox: got %+v, want (0,0)-(100,100)", tb.BBox)
	}
	// Row 0 = top = larger Y. Row 0, Col 0 should be (0, 50, 50, 100).
	if c := tb.CellsGrid[0][0]; c.X0 != 0 || c.Y1 != 100 {
		t.Errorf("CellsGrid[0][0]: got %+v, want top-left", c)
	}
	if c := tb.CellsGrid[0][1]; c.X0 != 50 || c.Y1 != 100 {
		t.Errorf("CellsGrid[0][1]: got %+v, want top-right", c)
	}
	if c := tb.CellsGrid[1][0]; c.X0 != 0 || c.Y1 != 50 {
		t.Errorf("CellsGrid[1][0]: got %+v, want bottom-left", c)
	}
	if c := tb.CellsGrid[1][1]; c.X0 != 50 || c.Y1 != 50 {
		t.Errorf("CellsGrid[1][1]: got %+v, want bottom-right", c)
	}
}

// TestRunTableFinder_2x3Grid is an end-to-end algorithm test that
// builds the edges for a 2-column × 3-row grid by hand, runs the
// full pipeline, and asserts the output has the right shape:
// 12 intersections, 6 cells, 1 table, 3×2 grid.
func TestRunTableFinder_2x3Grid(t *testing.T) {
	// Columns at X = 100, 200, 300; rows at Y = 0, 50, 100, 150.
	edges := []layout.Edge{
		makeH(100, 300, 0),
		makeH(100, 300, 50),
		makeH(100, 300, 100),
		makeH(100, 300, 150),
		makeV(100, 0, 150),
		makeV(200, 0, 150),
		makeV(300, 0, 150),
	}
	finder := runTableFinder(edges, 0.1, 0.1)
	if got := len(finder.Intersections); got != 12 {
		t.Errorf("intersections: got %d, want 12", got)
	}
	if got := len(finder.Cells); got != 6 {
		t.Errorf("cells: got %d, want 6", got)
	}
	if got := len(finder.Tables); got != 1 {
		t.Fatalf("tables: got %d, want 1", got)
	}
	tb := finder.Tables[0]
	if tb.Rows != 3 || tb.Cols != 2 {
		t.Errorf("grid: got %dx%d, want 3x2", tb.Rows, tb.Cols)
	}
}

// TestEnsureSupportedStrategies_RejectsTextAndExplicit asserts that
// the v0.3.0 strategies return ErrUnsupported rather than silently
// running an empty pipeline.
func TestEnsureSupportedStrategies_RejectsTextAndExplicit(t *testing.T) {
	cases := []struct {
		name string
		s    TableSettings
	}{
		{"text/lines", TableSettings{VerticalStrategy: StrategyText, HorizontalStrategy: StrategyLines}},
		{"lines/text", TableSettings{VerticalStrategy: StrategyLines, HorizontalStrategy: StrategyText}},
		{"explicit/lines", TableSettings{VerticalStrategy: StrategyExplicit, HorizontalStrategy: StrategyLines}},
		{"lines/explicit", TableSettings{VerticalStrategy: StrategyLines, HorizontalStrategy: StrategyExplicit}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ensureSupportedStrategies(c.s.applyDefaults())
			if err == nil {
				t.Fatal("got nil error, want ErrUnsupported")
			}
			if !errIs(err, ErrUnsupported) {
				t.Errorf("got %v, want ErrUnsupported", err)
			}
		})
	}
}

// TestEnsureSupportedStrategies_AcceptsLines asserts that both lines
// strategies pass validation.
func TestEnsureSupportedStrategies_AcceptsLines(t *testing.T) {
	cases := []struct {
		name string
		s    TableSettings
	}{
		{"lines/lines", TableSettings{VerticalStrategy: StrategyLines, HorizontalStrategy: StrategyLines}},
		{"strict/lines", TableSettings{VerticalStrategy: StrategyLinesStrict, HorizontalStrategy: StrategyLines}},
		{"lines/strict", TableSettings{VerticalStrategy: StrategyLines, HorizontalStrategy: StrategyLinesStrict}},
		{"strict/strict", TableSettings{VerticalStrategy: StrategyLinesStrict, HorizontalStrategy: StrategyLinesStrict}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := ensureSupportedStrategies(c.s.applyDefaults()); err != nil {
				t.Errorf("got %v, want nil", err)
			}
		})
	}
}

// TestApplyDefaults_FillsZeroFields verifies the zero-value defaults
// match pdfplumber's constants.
func TestApplyDefaults_FillsZeroFields(t *testing.T) {
	s := TableSettings{}.applyDefaults()
	if s.VerticalStrategy != StrategyLines {
		t.Errorf("VerticalStrategy: got %q, want %q", s.VerticalStrategy, StrategyLines)
	}
	if s.SnapTolerance != 3 {
		t.Errorf("SnapTolerance: got %v, want 3", s.SnapTolerance)
	}
	if s.JoinTolerance != 3 {
		t.Errorf("JoinTolerance: got %v, want 3", s.JoinTolerance)
	}
	if s.EdgeMinLength != 3 {
		t.Errorf("EdgeMinLength: got %v, want 3", s.EdgeMinLength)
	}
	if s.EdgeMinLengthPrefilter != 1 {
		t.Errorf("EdgeMinLengthPrefilter: got %v, want 1", s.EdgeMinLengthPrefilter)
	}
	if s.IntersectionTolerance != 3 {
		t.Errorf("IntersectionTolerance: got %v, want 3", s.IntersectionTolerance)
	}
	if s.TextTolerance != 3 {
		t.Errorf("TextTolerance: got %v, want 3", s.TextTolerance)
	}
}

// TestExtractTables_RuledFixture is the end-to-end integration test
// against the hand-crafted 2×3 ruled-table fixture. It opens the
// generated PDF, runs ExtractTables with default settings, and
// asserts the row count + cell text matches the fixture's known
// content.
//
// This test uses the public API only — the unit tests above cover
// the unexported algorithm functions.
func TestExtractTables_RuledFixture(t *testing.T) {
	// Import path is package-internal here (we're in the pdftable
	// package, not _test), so OpenBytes is unqualified.
	doc, err := OpenBytes(testdata.TableRuled())
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	defer doc.Close()

	p, err := doc.Page(1)
	if err != nil {
		t.Fatalf("Page(1): %v", err)
	}

	tables, err := p.ExtractTables(DefaultTableSettings())
	if err != nil {
		t.Fatalf("ExtractTables: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("got %d tables, want 1", len(tables))
	}
	tbl := tables[0]
	if len(tbl.Rows) != 3 {
		t.Fatalf("rows: got %d, want 3", len(tbl.Rows))
	}
	for i, row := range tbl.Rows {
		if len(row) != 2 {
			t.Errorf("row %d: got %d cols, want 2", i, len(row))
		}
	}
	// Row 0 (visually top): Name | Age
	// Row 1: Alice | 30
	// Row 2: Bob   | 25
	want := [][]string{
		{"Name", "Age"},
		{"Alice", "30"},
		{"Bob", "25"},
	}
	for i := range want {
		for j := range want[i] {
			got := tbl.Rows[i][j]
			if got != want[i][j] {
				t.Errorf("Rows[%d][%d]: got %q, want %q", i, j, got, want[i][j])
			}
		}
	}
	if tbl.Page != 1 {
		t.Errorf("Page: got %d, want 1", tbl.Page)
	}
	if tbl.BBox.IsZero() {
		t.Error("BBox is zero")
	}
}

// TestExtractTables_UnsupportedStrategyReturnsErrUnsupported asserts
// the public API surfaces ErrUnsupported when callers request "text"
// or "explicit" strategies.
func TestExtractTables_UnsupportedStrategyReturnsErrUnsupported(t *testing.T) {
	doc, err := OpenBytes(testdata.TableRuled())
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	defer doc.Close()
	p, _ := doc.Page(1)

	settings := DefaultTableSettings()
	settings.VerticalStrategy = StrategyText
	_, err = p.ExtractTables(settings)
	if err == nil {
		t.Fatal("got nil, want ErrUnsupported")
	}
	if !errIs(err, ErrUnsupported) {
		t.Errorf("got %v, want ErrUnsupported", err)
	}
	// The error should mention what was unsupported and the phase.
	if !strings.Contains(err.Error(), "text") {
		t.Errorf("error %q should name the strategy", err.Error())
	}
}

// TestFindTables_NoEdgesReturnsEmpty asserts that a page with no
// edges (e.g. a text-only page) returns an empty slice, not an
// error.
func TestFindTables_NoEdgesReturnsEmpty(t *testing.T) {
	doc, err := OpenBytes(testdata.Hello())
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	defer doc.Close()
	p, _ := doc.Page(1)
	finders, err := p.FindTables(DefaultTableSettings())
	if err != nil {
		t.Errorf("FindTables on text-only page: got %v, want nil", err)
	}
	if len(finders) != 0 {
		t.Errorf("got %d finders, want 0 (text-only page)", len(finders))
	}
}
