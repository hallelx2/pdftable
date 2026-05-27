# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-05-27

Phase 1.3.C — table-finding via ruled lines. Direct port of
pdfplumber's `TableFinder` + cells-from-edges algorithm (`table.py`).
The v0.1.x public API surface is unchanged; v0.2.0 only adds methods
to the `Page` interface and new top-level types, so existing callers
compile and run as-is.

### Added

- `Page.FindTables(settings TableSettings) ([]TableFinder, error)` —
  geometry-only stage of the pipeline. Returns one TableFinder per
  detected table group with the merged edges, intersections, raw
  cells, and assembled per-table CellsGrid exposed for debugging /
  custom rendering.
- `Page.ExtractTables(settings TableSettings) ([]*Table, error)` —
  wraps FindTables, runs per-cell text extraction, returns fully
  populated `Table` structs. Cell text is the dense extract\_text
  output for chars whose centre point falls inside the cell bbox,
  with leading / trailing whitespace stripped. Empty cells produce
  `""`.
- `TableSettings` struct with `DefaultTableSettings()` constructor
  carrying pdfplumber-matching defaults (snap\_tolerance=3,
  join\_tolerance=3, edge\_min\_length=3, edge\_min\_length\_prefilter=1,
  intersection\_tolerance=3, text\_tolerance=3).
- `TableStrategy` enum with constants `StrategyLines`,
  `StrategyLinesStrict`, `StrategyText`, `StrategyExplicit`. Only
  `StrategyLines` and `StrategyLinesStrict` are implemented in this
  release; `StrategyText` and `StrategyExplicit` are deferred to
  v0.3.0 and return `ErrUnsupported` (with a clear "Phase 1.3.D"
  message) so callers don't get silent empty results.
- `Table` (rows × columns of cell text + bbox + per-cell bbox grid),
  `TableFinder` (edges + intersections + cells + tables), `TableBox`
  (one assembled table's geometry: bbox + Rows × Cols grid),
  `Intersection` (one edge-crossing point with its participating
  vertical and horizontal edges).
- Internal `internal/layout` package: `Edge` type with `FromLine`,
  `FromRect`, `FromCurve` constructors, plus the snap → join →
  filter pipeline (`SnapEdges`, `JoinEdges`, `MergeEdges`,
  `FilterEdgesByLength`, `FilterEdgesBySource`,
  `FilterEdgesByOrientation`, `SortEdges`).
- Golden-file parity test against pdfplumber's `find_tables({"lines"})`
  on the `issue-466-example.pdf` fixture (4×3 + 2×3 ruled tables).
  Test infrastructure (`TestGoldenTablesAgainstPdfplumber` in
  `golden_test.go`) loads any `*.tables.expected.json` fixture in
  `testdata/golden/` and compares cell-for-cell after whitespace
  normalisation. Regenerate via `python scripts/gen_golden.py`.
- New hand-crafted fixture: `testdata.TableRuled()` — minimal
  2-column × 3-row ruled table with predictable text ("Name", "Age";
  "Alice", "30"; "Bob", "25") for unit testing the public API
  surface without depending on third-party PDFs. Generator script
  at `scripts/gen_table_fixture.go`.
- Algorithm-level unit tests in `table_test.go`: hand-crafted edge
  lists exercising `edgesToIntersections`, `intersectionsToCells`,
  `cellsToTables`, `assembleTableBox`, and the full `runTableFinder`
  pipeline.
- README "Tables" section with a side-by-side Go / pdfplumber
  example. The example is also extracted as a runnable program at
  `examples/extract_tables/main.go` so changes to the API surface
  break the example at build time.

### Deferred (planned for v0.3.0 — Phase 1.3.D)

- `StrategyText`: infer table edges from word alignment (clusters of
  words sharing x0 / x1 / centre, clusters of words sharing top /
  bottom). Useful for PDFs whose tables have no ruled lines (e.g.
  banking statements, scanned-then-OCR'd documents).
- `StrategyExplicit`: caller-supplied edges via
  `TableSettings.ExplicitVerticalLines` /
  `ExplicitHorizontalLines`. In v0.2.0 these settings are accepted
  and added on top of the derived edges (helpful when a column
  boundary isn't drawn), but they don't form the only source of
  edges yet.

### Known limitations

- The cell-text extraction shares the v0.1.x word-grouping engine,
  which depends on font metrics. Cells whose glyphs use standard-14
  fonts WITHOUT the bundled AFM tables can have intra-word gaps
  reported as "no gap" — e.g. "Hello World" comes out as
  "HelloWorld". This was already documented for v0.1.0; for v0.2.0
  it means the parity test against
  `la-precinct-bulletin-2014-p1.pdf` (which uses Helvetica-Bold)
  fails on cell text equality. The fixture is not checked in to
  avoid CI noise; it'll be re-added once the AFM bundle lands in
  v0.2.x.
- `senate-expenditures.pdf` produces 7 cells where pdfplumber finds
  10. The divergence is in how snap+join unifies edges that share a
  near-collinear endpoint but differ slightly in the perpendicular
  axis; under investigation as a follow-up issue. The fixture is
  not in the golden set yet.

## [0.1.1] - 2026-05-27

### Fixed

- StandardEncoding, WinAnsiEncoding, MacRomanEncoding, and
  PDFDocEncoding are now driven from a single source of truth
  (`encodingRows`) that mirrors pdfminer.six's `latin_enc.py` and PDF
  Reference 1.7 Appendix D.2. The previous tables silently dropped
  ~32 named glyphs per encoding outside printable ASCII — most
  visibly the smart quotes (`’ ‘ “ ”`), en/em dashes (`– —`),
  bullet (`•`), florin (`ƒ`), and dagger marks (`† ‡`). PDFs that
  used these without a `/ToUnicode` map (the common case for PDF/A
  filings, SEC 10-Ks, and most LaTeX-emitted documents) returned
  empty or garbled text where these glyphs appeared.
- `AdobeGlyphToUnicode` now resolves the full Adobe Glyph List for
  common Latin/typographic glyphs (~250 entries) instead of a minimal
  ~30-entry table. Added support for AGL §2 compound names (`f_i`
  decomposes to `fi`) and variant suffixes (`.alt`, `.sc` are
  stripped before lookup).
- StandardEncoding now correctly maps slot 0x27 to `quoteright`
  (`’`, U+2019) and 0x60 to `quoteleft` (`‘`, U+2018), matching the
  PDF spec. WinAnsi/MacRoman/PDFDoc keep ASCII `'` and `` ` `` at
  those slots, as the spec requires.

### Note

This is a behavior change for callers that depended on the pre-v0.1.1
ASCII-identity behavior of StandardEncoding at 0x27 / 0x60. The new
behavior is spec-correct and matches what pdfplumber, pdfminer.six,
and Ghostscript emit for the same input.

## [0.1.0] - 2026-05-26

Phase 1.3.B — words and text extraction. Direct port of pdfplumber's
`WordExtractor`, `extract_text`, `extract_text_simple`. The v0.0.1
public API surface is unchanged; v0.1.0 only adds methods to the
`Page` interface, so existing callers compile and run as-is.

### Added

- `Page.Words(opts WordOpts) ([]Word, error)` — extract positioned
  text runs. Each `Word` carries `Text`, `X0/Y0/X1/Y1` bbox,
  `Upright`, `Direction` (ltr/rtl/ttb/btt), `FontName`, `FontSize`,
  and an optional `Chars` slice (when `WordOpts.KeepChars=true`).
- `Page.ExtractText(opts TextOpts) (string, error)` — page text as a
  single string. Supports both dense (`Layout=false`, the default)
  and layout-preserving (`Layout=true`) modes. The layout mode emits
  a fixed-width grid mimicking `pdftotext -layout` / pdfplumber's
  `extract_text(layout=True)`.
- `Page.ExtractTextSimple(xTolerance, yTolerance float64) (string, error)` —
  no-frills extraction baseline (ports pdfplumber's
  `extract_text_simple`).
- `WordOpts` / `TextOpts` option structs with `DefaultWordOpts()` /
  `DefaultTextOpts()` constructors carrying pdfplumber-matching
  defaults (XTolerance=3, YTolerance=3, Expand=true).
- `BBox` value type with `Union`, `Intersect`, `Contains`, `Snap`,
  `MergeBBoxes`, `BBoxOfChar`, `BBoxOfChars` helpers.
- Internal clustering primitives in `clustering.go`:
  `clusterFloat1D`, `makeClusterDict`, `clusterObjects[T]`,
  `groupObjectsByAttr[T,K]`, `dedupeChars`. Ports of
  pdfplumber/utils/clustering.py.
- Ligature expansion table (ﬁ, ﬂ, ﬀ, ﬃ, ﬄ, ﬅ, ﬆ → fi/fl/ff/ffi/ffl/st).
- Golden-file parity tests against pdfplumber output on three
  fixtures (hello.pdf, rules.pdf, simple1.pdf). Regenerate via
  `python scripts/gen_golden.py`.

### Known limitations

- Word bboxes drift by up to ~10 PDF points from pdfplumber's output
  on standard-14 fonts because the AFM metrics aren't yet bundled.
  Word text + count + order match exactly. The AFM bundle is a v0.2.x
  goal.
- `extract_text_lines` (regex-based line extraction) is not yet
  ported.
- `TextMap.search` is not yet ported.

## [0.0.1] - 2026-05-26

Initial release. Phase 1.3.A — content-stream primitives layer.

### Added

- Public API: `Open`, `OpenBytes`, `OpenFile` → `Document`.
- `Document.NumPages`, `Document.Page(n)`, `Document.Pages()` iterator,
  `Document.Close`.
- `Page.Number`, `Page.Width`, `Page.Height`, `Page.Chars`, `Page.Lines`,
  `Page.Rects`, `Page.Curves`, `Page.Objects`.
- `Char`, `Line`, `Rect`, `Curve`, `Objects` data types with PDF-user-space
  coordinates (origin at bottom-left, Y growing upwards), normalised
  bounding boxes (`x0 <= x1`, `y0 <= y1`).
- Sentinel errors: `ErrInvalidPDF`, `ErrPageOutOfRange`, `ErrUnsupported`,
  `ErrEncrypted`.
- Internal content-stream interpreter (`internal/pdf`) — port of
  pdfminer.six's `PDFContentEmitter`:
  - Graphics-state stack (`q`/`Q`/`cm`/`w`).
  - Path construction (`m`/`l`/`c`/`v`/`y`/`h`/`re`).
  - Path painting (`S`/`s`/`f`/`F`/`f*`/`B`/`B*`/`b`/`b*`/`n`).
  - Text state (`Tc`/`Tw`/`Tz`/`TL`/`Tf`/`Tr`/`Ts`).
  - Text positioning (`BT`/`ET`/`Td`/`TD`/`Tm`/`T*`).
  - Text showing (`Tj`/`TJ`/`'`/`"`).
  - Form XObject inlining (`Do`).
- ToUnicode CMap parser (`bfchar`, `bfrange` with hex-string and array
  destinations, UTF-16BE decoding including surrogate pairs).
- Font subsystem: simple Type1/TrueType fonts with `/Encoding` and
  `/Differences` overlays; composite Type0 fonts with descendant
  CIDFont width arrays (`/W` short and range forms, `/DW`).
- Standard PDF encodings: StandardEncoding, WinAnsiEncoding,
  MacRomanEncoding, PDFDocEncoding (printable-ASCII identity + WinAnsi
  high-range smart quotes / dashes / euro).
- Adobe glyph-name table for common glyphs + `uni`/`u` recognisers for
  the rest.
- Test fixtures generated programmatically (`testdata/fixtures.go`):
  hand-crafted Hello-World and ruling-lines PDFs.

### Out of scope (later phases)

- `Page.ExtractText`, `Page.Words` (1.3.B).
- `Page.FindTables`, `Page.ExtractTables` (1.3.C/D).
- Encrypted PDF handling.
- Type 3 fonts (their glyph procedures are themselves content streams).
- Vertical writing mode.

[0.2.0]: https://github.com/hallelx2/pdftable/releases/tag/v0.2.0
[0.1.1]: https://github.com/hallelx2/pdftable/releases/tag/v0.1.1
[0.1.0]: https://github.com/hallelx2/pdftable/releases/tag/v0.1.0
[0.0.1]: https://github.com/hallelx2/pdftable/releases/tag/v0.0.1
