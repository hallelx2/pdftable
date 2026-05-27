# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.1.1]: https://github.com/hallelx2/pdftable/releases/tag/v0.1.1
[0.1.0]: https://github.com/hallelx2/pdftable/releases/tag/v0.1.0
[0.0.1]: https://github.com/hallelx2/pdftable/releases/tag/v0.0.1
