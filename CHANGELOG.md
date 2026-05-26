# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.0.1]: https://github.com/hallelx2/pdftable/releases/tag/v0.0.1
