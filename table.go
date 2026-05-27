// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdftable

// This file defines the public types of the table-finding pipeline:
// TableSettings (with pdfplumber-matching defaults), Table (the
// extracted result), and TableFinder (the intermediate object that
// exposes edges, intersections, and cell bboxes without running text
// extraction).
//
// Algorithm and field names are direct ports of pdfplumber's
// TableSettings / TableFinder / Table from pdfplumber/table.py. The
// public surface differs in two ways:
//
//   - Field names follow Go conventions (CamelCase, exported) rather
//     than pdfplumber's snake_case dict keys.
//   - Coordinates are PDF user space (origin at bottom-left, Y growing
//     up). pdfplumber emits image-space coordinates ("top" / "bottom"
//     with Y growing down); we use Y0/Y1 throughout. The intersection
//     geometry is invariant under that flip; only the comments
//     describing "below" / "right" change their sign.

// TableStrategy is the enum of edge-derivation strategies. Each axis
// (vertical, horizontal) picks one. v0.2.0 implements "lines" and
// "lines_strict"; "text" and "explicit" are reserved for the next
// release (Phase 1.3.D) and return ErrUnsupported if requested.
type TableStrategy string

const (
	// StrategyLines derives edges from drawn Lines, Rects (all four
	// sides), and Curves whose segments lie on an axis. Snap and join
	// tolerances are at their defaults — looser than lines_strict so
	// hand-drawn or jittery rules still merge.
	StrategyLines TableStrategy = "lines"

	// StrategyLinesStrict derives edges ONLY from drawn Lines.
	// Rectangle outlines and curve segments are ignored, even if they
	// look like a table grid. Use this when your PDF draws cell
	// backgrounds as filled rects that you do NOT want treated as row
	// boundaries.
	StrategyLinesStrict TableStrategy = "lines_strict"

	// StrategyText (Phase 1.3.D) infers edges from word alignment.
	StrategyText TableStrategy = "text"

	// StrategyExplicit (Phase 1.3.D) uses caller-supplied lines.
	StrategyExplicit TableStrategy = "explicit"
)

// TableSettings controls table finding. Construct via
// DefaultTableSettings() and override the fields you need — the zero
// value is NOT usable because the tolerances default to zero and the
// strategies are empty strings.
//
// Field naming and defaults are 1:1 with pdfplumber's TableSettings
// dataclass (see pdfplumber/table.py:486-555). Where pdfplumber
// supports independent x/y tolerances via *_x_tolerance / *_y_tolerance
// fallbacks, we expose the shared field directly; explicit per-axis
// overrides can be added later if a real-world need surfaces.
type TableSettings struct {
	// VerticalStrategy picks the source of vertical edges.
	// Default: StrategyLines.
	VerticalStrategy TableStrategy

	// HorizontalStrategy picks the source of horizontal edges.
	// Default: StrategyLines.
	HorizontalStrategy TableStrategy

	// SnapTolerance is the perpendicular-axis tolerance for clustering
	// near-collinear edges before joining (PDF points). Default: 3.
	SnapTolerance float64

	// JoinTolerance is the along-direction gap that still gets merged
	// during the join pass (PDF points). Default: 3.
	JoinTolerance float64

	// EdgeMinLength drops merged edges shorter than this (PDF points).
	// Default: 3.
	EdgeMinLength float64

	// EdgeMinLengthPrefilter drops raw edges before merging
	// (PDF points). Default: 1 — kills hairline construction
	// segments that snap+join shouldn't pull together.
	EdgeMinLengthPrefilter float64

	// IntersectionTolerance is the slack used when testing whether a
	// vertical edge crosses a horizontal edge — accounts for tiny
	// gaps between the end of a stroked line and the start of the
	// next (PDF points). Default: 3.
	IntersectionTolerance float64

	// TextTolerance is forwarded to the per-cell text-extraction call
	// inside ExtractTables. It overrides both x_tolerance and
	// y_tolerance of the underlying WordExtractor. Default: 3.
	TextTolerance float64

	// MinWordsVertical / MinWordsHorizontal control the "text"
	// strategy thresholds (Phase 1.3.D). They have no effect when
	// both strategies are "lines" / "lines_strict" — kept on this
	// struct so callers don't have to switch types when migrating to
	// the text strategy later.
	MinWordsVertical   int
	MinWordsHorizontal int

	// KeepBlankChars is forwarded to the per-cell WordExtractor.
	// Default: false (matches pdfplumber's text_keep_blank_chars).
	KeepBlankChars bool

	// ExplicitVerticalLines / ExplicitHorizontalLines hold caller-
	// supplied edge positions. With StrategyLines or
	// StrategyLinesStrict they are ADDED to the derived edges; with
	// StrategyExplicit they ARE the edges. In v0.2.0 the explicit
	// strategy is not yet implemented; these slices have effect only
	// when one of the lines strategies is in use and you want extra
	// hand-placed rules (e.g. when your column boundary isn't drawn).
	//
	// Values are X coordinates for vertical lines, Y coordinates for
	// horizontal lines, both in PDF user-space points.
	ExplicitVerticalLines   []float64
	ExplicitHorizontalLines []float64
}

// DefaultTableSettings returns settings with the pdfplumber default
// values pre-populated. The intended pattern is:
//
//	settings := pdftable.DefaultTableSettings()
//	settings.VerticalStrategy = pdftable.StrategyLinesStrict
//	tables, err := page.ExtractTables(settings)
//
// pdfplumber's defaults (table.py lines 9-12, 486-503):
//
//	DEFAULT_SNAP_TOLERANCE         = 3
//	DEFAULT_JOIN_TOLERANCE         = 3
//	DEFAULT_MIN_WORDS_VERTICAL     = 3
//	DEFAULT_MIN_WORDS_HORIZONTAL   = 1
//	edge_min_length                = 3
//	edge_min_length_prefilter      = 1
//	intersection_tolerance         = 3
//	vertical_strategy              = "lines"
//	horizontal_strategy            = "lines"
//	text_x_tolerance/y_tolerance   = 3
func DefaultTableSettings() TableSettings {
	return TableSettings{
		VerticalStrategy:       StrategyLines,
		HorizontalStrategy:     StrategyLines,
		SnapTolerance:          3,
		JoinTolerance:          3,
		EdgeMinLength:          3,
		EdgeMinLengthPrefilter: 1,
		IntersectionTolerance:  3,
		TextTolerance:          3,
		MinWordsVertical:       3,
		MinWordsHorizontal:     1,
	}
}

// applyDefaults fills in zero-valued fields with pdfplumber-matching
// defaults. Callers who construct a TableSettings literal and only set
// the fields they care about get the same defaults as if they'd used
// DefaultTableSettings().
func (s TableSettings) applyDefaults() TableSettings {
	if s.VerticalStrategy == "" {
		s.VerticalStrategy = StrategyLines
	}
	if s.HorizontalStrategy == "" {
		s.HorizontalStrategy = StrategyLines
	}
	if s.SnapTolerance == 0 {
		s.SnapTolerance = 3
	}
	if s.JoinTolerance == 0 {
		s.JoinTolerance = 3
	}
	if s.EdgeMinLength == 0 {
		s.EdgeMinLength = 3
	}
	if s.EdgeMinLengthPrefilter == 0 {
		s.EdgeMinLengthPrefilter = 1
	}
	if s.IntersectionTolerance == 0 {
		s.IntersectionTolerance = 3
	}
	if s.TextTolerance == 0 {
		s.TextTolerance = 3
	}
	if s.MinWordsVertical == 0 {
		s.MinWordsVertical = 3
	}
	if s.MinWordsHorizontal == 0 {
		s.MinWordsHorizontal = 1
	}
	return s
}

// Table is the extracted result for one detected table. It carries
// the assembled cell texts plus the geometry needed for downstream
// consumers (re-rendering, click-through to source positions).
type Table struct {
	// Rows is the table's text content as a 2-D slice. Row 0 is the
	// VISUALLY TOP row of the table; column 0 is the leftmost. Empty
	// cells appear as "". Missing cells (when a row has fewer columns
	// than the table's column count, because the underlying cell
	// detection found a hole) are also "" — we promote missing to
	// empty so callers don't have to nil-check every entry.
	Rows [][]string

	// BBox is the union of every cell's bbox, in PDF user-space
	// coordinates (origin bottom-left, Y growing up).
	BBox BBox

	// Page is the 1-based page number the table was found on, copied
	// from the originating Page so callers can carry results across
	// page boundaries without holding Page references.
	Page int

	// CellsBBox is the per-cell bbox aligned to Rows: CellsBBox[i][j]
	// is the bbox of Rows[i][j]. Useful for re-rendering with
	// highlight overlays, or for re-cropping the page to extract the
	// cell's contents in a richer format than plain text.
	CellsBBox [][]BBox
}

// Cells returns the cell bboxes flattened into reading order
// (left-to-right, top-to-bottom). Provided as a convenience for
// callers that want a single iterable rather than a nested slice.
func (t Table) Cells() []BBox {
	out := make([]BBox, 0)
	for _, row := range t.CellsBBox {
		out = append(out, row...)
	}
	return out
}
