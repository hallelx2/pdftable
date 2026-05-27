# pdftable

A Go-native port of Python's [pdfplumber](https://github.com/jsvine/pdfplumber).

`pdftable` reads PDF documents, walks the content streams, and surfaces
the positioned primitives — characters, lines, rectangles, curves — that
higher-level layout algorithms (text extraction, word grouping, table
detection) operate on. It is built on top of
[pdfcpu](https://github.com/pdfcpu/pdfcpu) for low-level object parsing,
xref handling, and FlateDecode decompression; everything above that
(operator dispatch, text state, glyph positioning, ToUnicode CMaps,
font encodings) is implemented here.

The library targets the gap in the Go PDF ecosystem: existing libraries
either render PDFs to images, manipulate metadata, or extract bag-of-
words text. None of them give you what pdfplumber gives Python users —
a structured per-page object model you can run table-detection
heuristics on. This is that.

## Status

`v0.2.0` — line-strategy table finding. `Page.FindTables` and
`Page.ExtractTables` ship with this release covering the `lines` and
`lines_strict` strategies (PDFs with ruled tables). `text` and
`explicit` strategies return `ErrUnsupported` and land in v0.3.0.

[![Go Reference](https://pkg.go.dev/badge/github.com/hallelx2/pdftable.svg)](https://pkg.go.dev/github.com/hallelx2/pdftable)
[![CI](https://github.com/hallelx2/pdftable/actions/workflows/test.yml/badge.svg)](https://github.com/hallelx2/pdftable/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Install

```sh
go get github.com/hallelx2/pdftable@v0.2.0
```

Requires Go 1.25+ (uses the standard-library `iter` package for the `Pages()` range-over-func iterator, and pdfcpu v0.12+).

## Quickstart

```go
package main

import (
    "fmt"
    "log"

    "github.com/hallelx2/pdftable"
)

func main() {
    doc, err := pdftable.OpenFile("report.pdf")
    if err != nil {
        log.Fatal(err)
    }
    defer doc.Close()

    for n, page := range doc.Pages() {
        // Primitives (v0.0.1).
        chars, _ := page.Chars()
        rects, _ := page.Rects()
        lines, _ := page.Lines()
        fmt.Printf("page %d: %d chars, %d rects, %d lines\n",
            n, len(chars), len(rects), len(lines))

        // Words and text extraction (v0.1.0).
        words, _ := page.Words(pdftable.DefaultWordOpts())
        text, _ := page.ExtractText(pdftable.DefaultTextOpts())
        fmt.Printf("  %d words; first line: %q\n",
            len(words), firstLine(text))
    }
}

func firstLine(s string) string {
    for i, r := range s {
        if r == '\n' {
            return s[:i]
        }
    }
    return s
}
```

## API surface

```go
// Constructors.
func Open(r io.Reader) (Document, error)
func OpenBytes(b []byte) (Document, error)
func OpenFile(path string) (Document, error)

// Document.
type Document interface {
    NumPages() int
    Page(n int) (Page, error)              // 1-indexed
    Pages() iter.Seq2[int, Page]           // Go 1.23+ range-over-func
    Close() error
}

// Page.
type Page interface {
    Number() int
    Width() float64
    Height() float64
    Chars() ([]Char, error)
    Lines() ([]Line, error)
    Rects() ([]Rect, error)
    Curves() ([]Curve, error)
    Objects() (Objects, error)

    // New in v0.1.0: word + text extraction.
    Words(opts WordOpts) ([]Word, error)
    ExtractText(opts TextOpts) (string, error)
    ExtractTextSimple(xTolerance, yTolerance float64) (string, error)

    // New in v0.2.0: line-strategy table finding.
    FindTables(settings TableSettings) ([]TableFinder, error)
    ExtractTables(settings TableSettings) ([]*Table, error)
}

// Primitives.
type Char struct {
    Text                  string
    X0, Y0, X1, Y1        float64
    FontName              string
    FontSize              float64
    Upright               bool
    Advance               float64
}

type Line struct { X0, Y0, X1, Y1 float64; Stroke bool; Width float64 }

type Rect struct { X0, Y0, X1, Y1 float64; Stroke, Fill bool; Width float64 }

type Curve struct { Points [][2]float64; Stroke, Fill bool; Width float64 }

type Objects struct { Chars []Char; Lines []Line; Rects []Rect; Curves []Curve }

// Word (new in v0.1.0).
type Word struct {
    Text                string
    X0, Y0, X1, Y1      float64
    Upright             bool
    Direction           string // "ltr" | "rtl" | "ttb" | "btt"
    FontName            string
    FontSize            float64
    Chars               []Char // populated when WordOpts.KeepChars=true
}

// WordOpts: configure Page.Words. Use DefaultWordOpts() for pdfplumber-matching defaults.
type WordOpts struct {
    XTolerance         float64 // default 3
    YTolerance         float64 // default 3
    KeepBlankChars     bool
    UseTextFlow        bool
    HorizontalLTR      bool   // default true
    VerticalTTB        bool   // default true
    ExtraAttrs         []string
    SplitAtPunctuation bool
    Expand             bool   // ligature expansion; default true
    KeepChars          bool
}

// TextOpts: configure Page.ExtractText. Use DefaultTextOpts() for defaults.
type TextOpts struct {
    XTolerance, YTolerance       float64
    Layout                       bool
    LayoutWidthChars             int
    LayoutHeightChars            int
    XDensity, YDensity           float64 // PDF points per character / per line
    UseTextFlow                  bool
    HorizontalLTR                bool
    VerticalTTB                  bool
    ExtraAttrs                   []string
    Expand                       bool
}

// Sentinel errors.
var (
    ErrInvalidPDF     = errors.New("pdftable: invalid PDF")
    ErrPageOutOfRange = errors.New("pdftable: page out of range")
    ErrUnsupported    = errors.New("pdftable: unsupported feature")
    ErrEncrypted      = errors.New("pdftable: encrypted PDF (decryption not yet supported)")
)
```

## Text extraction

```go
doc, _ := pdftable.OpenFile("report.pdf")
defer doc.Close()
page, _ := doc.Page(1)

// Words: each Word is a contiguous text run.
words, _ := page.Words(pdftable.DefaultWordOpts())
for _, w := range words {
    fmt.Printf("%-20s @ (%.1f, %.1f) %s %.1fpt\n",
        w.Text, w.X0, w.Y0, w.FontName, w.FontSize)
}

// ExtractText: all text on the page as one string. Dense (no layout)
// joins words with spaces and lines with "\n".
text, _ := page.ExtractText(pdftable.DefaultTextOpts())
fmt.Println(text)

// Layout-preserving extraction emulates `pdftotext -layout` / pdfplumber's
// extract_text(layout=True) — column-aligned output suitable for forms.
opts := pdftable.DefaultTextOpts()
opts.Layout = true
laid, _ := page.ExtractText(opts)
fmt.Println(laid)
```

## Tables (lines strategy)

`Page.ExtractTables` is the table-detection entry point. It runs the
edges → intersections → cells → tables pipeline (a direct port of
pdfplumber's `TableFinder`) and returns one `*Table` per detected
ruled table, with cell text already extracted.

```go
doc, _ := pdftable.OpenFile("invoice.pdf")
defer doc.Close()
page, _ := doc.Page(1)

settings := pdftable.DefaultTableSettings()
// settings.VerticalStrategy = pdftable.StrategyLinesStrict  // ignore rect outlines

tables, _ := page.ExtractTables(settings)
for ti, t := range tables {
    fmt.Printf("table %d: %d rows × %d cols at %+v\n",
        ti, len(t.Rows), len(t.Rows[0]), t.BBox)
    for _, row := range t.Rows {
        fmt.Println(row)
    }
}
```

`TableSettings` defaults match pdfplumber's
(`snap_tolerance=3`, `join_tolerance=3`, `edge_min_length=3`,
`intersection_tolerance=3`, `text_tolerance=3`). Override any field
on the value returned from `DefaultTableSettings()` to tighten or
loosen the heuristics. The two implemented strategies are:

- `StrategyLines` — edges come from drawn `Line` segments, `Rect`
  outlines (all four sides), and axis-aligned `Curve` segments.
  Default. Best for typical PDFs whose tables have rule lines.
- `StrategyLinesStrict` — only drawn `Line` segments are used. Use
  this when your PDF draws cell BACKGROUNDS as filled rectangles
  that you do NOT want treated as row boundaries.

`StrategyText` (word-alignment-based) and `StrategyExplicit`
(caller-supplied edges) return `ErrUnsupported` in v0.2.0 — they
land in v0.3.0.

### Side-by-side: pdfplumber → pdftable

```python
# Python (pdfplumber)
import pdfplumber

with pdfplumber.open("invoice.pdf") as pdf:
    page = pdf.pages[0]
    for table in page.find_tables({"vertical_strategy": "lines",
                                    "horizontal_strategy": "lines"}):
        for row in table.extract():
            print(row)
```

```go
// Go (pdftable)
import "github.com/hallelx2/pdftable"

doc, _ := pdftable.OpenFile("invoice.pdf")
defer doc.Close()
page, _ := doc.Page(1)

settings := pdftable.DefaultTableSettings()
settings.VerticalStrategy = pdftable.StrategyLines
settings.HorizontalStrategy = pdftable.StrategyLines

tables, _ := page.ExtractTables(settings)
for _, t := range tables {
    for _, row := range t.Rows {
        fmt.Println(row)
    }
}
```

The two outputs match cell-for-cell on ruled fixtures (see
`testdata/golden/issue-466-example.*` for the parity test). Field
naming differs in the obvious places: pdftable returns a slice of
`*Table` instead of `Table` objects you have to call `.extract()`
on; rows are `[]string` instead of `list[Optional[str]]` (missing
cells produce `""` rather than `nil`); and table bboxes use
`(X0, Y0, X1, Y1)` PDF user space rather than pdfplumber's
image-space `(x0, top, x1, bottom)`.

## Side-by-side comparison with pdfplumber

```python
# Python (pdfplumber)
import pdfplumber

with pdfplumber.open("report.pdf") as pdf:
    page = pdf.pages[0]
    for word in page.extract_words(x_tolerance=3, y_tolerance=3):
        print(word["text"], word["x0"], word["top"])
    print(page.extract_text())
```

```go
// Go (pdftable)
import "github.com/hallelx2/pdftable"

doc, _ := pdftable.OpenFile("report.pdf")
defer doc.Close()
page, _ := doc.Page(1)

words, _ := page.Words(pdftable.DefaultWordOpts())
for _, w := range words {
    // pdftable's Y is PDF user-space (origin bottom-left). The
    // pdfplumber-equivalent "top" is page.Height() - w.Y1.
    fmt.Println(w.Text, w.X0, page.Height()-w.Y1)
}
fmt.Println(must(page.ExtractText(pdftable.DefaultTextOpts())))
```

Three differences worth noting:

1. **Page indexing is 1-based**, matching the PDF spec and pdfplumber's
   `pdf.pages[0]` is actually the first page (Python is 0-indexed,
   pdfplumber compensates). Our `Page(1)` is the same first page.
2. **Coordinates are in PDF user space with origin at bottom-left**.
   pdfplumber by default reports `top` (origin top-left, Y growing down)
   on its chars and words; we report `Y0` / `Y1` in PDF native
   coordinates. The conversion is `top = page.Height() - Y1`.
3. **Options are explicit Go structs, not `**kwargs`**. Build a
   `WordOpts` / `TextOpts`, override the fields you care about, pass
   it through. `DefaultWordOpts()` / `DefaultTextOpts()` return
   pdfplumber-matching defaults.

## Parity with pdfplumber

The word-grouping and text-extraction algorithms are direct ports of
pdfplumber's `WordExtractor` and `extract_text` (see
[`pdfplumber/utils/text.py`](https://github.com/jsvine/pdfplumber/blob/main/pdfplumber/utils/text.py)).
Tests in [`golden_test.go`](golden_test.go) compare the Go output
against pdfplumber's reference output on shared fixture PDFs.

Behaviours that match exactly:

- Word grouping: same line-cluster-then-merge-by-gap algorithm, same
  defaults (XTolerance=3, YTolerance=3), same handling of blank-char
  filtering, ligature expansion (ﬁ→fi, etc.), and split-at-punctuation.
- Ordering: words returned in pdfplumber's order (top-to-bottom, then
  left-to-right within each line) when UseTextFlow is false.
- Direction handling: ltr / rtl / ttb / btt mapping from
  upright + HorizontalLTR + VerticalTTB.

Behaviours that intentionally differ:

- **Position precision drifts when font metrics aren't bundled**.
  pdfplumber uses pdfminer.six's AFM tables for the standard 14 fonts;
  we use a default-width fallback for now. Word text and order match
  exactly; word bboxes drift by up to ~10 PDF points on glyphs whose
  width isn't in the PDF's /Widths array. Golden tests assert text
  parity exactly and position parity within a 15-point envelope; the
  envelope tightens to <1pt once the AFM bundle lands (planned for
  v0.2.x).
- **`Layout=true` output is structurally similar but not byte-equal**.
  Pdfplumber's layout algorithm has version-to-version drift; we
  produce a column-aligned grid with the same density defaults but
  don't promise byte-equal output across pdfplumber releases.

Behaviours not yet ported:

- `extract_text_lines` (regex-based line extraction).
- `search` on TextMap (regex over assembled page text with char-level
  match back-references).
- Per-character extra_attrs hooks beyond `fontname` and `size`.

## Architecture

```
pdftable/
├── pdftable.go        // Open / OpenBytes / OpenFile entry points
├── pdf.go             // Document interface + implementation
├── page.go            // Page interface + implementation
├── char.go            // Public Char / Line / Rect / Curve / Objects
├── text.go            // Word + ExtractText + ExtractTextSimple (v0.1.0)
├── table.go           // TableStrategy / TableSettings / Table types (v0.2.0)
├── finder.go          // Cells-from-edges algorithm (v0.2.0)
├── clustering.go      // 1-D clusterObjects, groupObjectsByAttr, dedupeChars
├── geometry.go        // BBox helpers: Union, Intersect, Contains, Snap
├── errors.go          // Sentinel errors
└── internal/
    ├── layout/
    │   └── lines.go   // Edge type + snap/join/filter pipeline (v0.2.0)
    └── pdf/
        ├── reader.go      // pdfcpu bridge
        ├── content.go     // Content-stream interpreter
        ├── ops.go         // Operator dispatch table
        ├── state.go       // Graphics + text state, matrix math
        ├── font.go        // Font + encoding tables + glyph-name resolution
        └── cmap.go        // ToUnicode CMap parser
```

The public `pdftable` package is small and stable. The `internal/pdf`
package owns the interpreter — its types are not exposed because they
will evolve as more PDF features are added (Type 3 fonts, vertical
writing, more exotic CMaps).

## Why pdfcpu and not write a PDF parser from scratch?

PDF object parsing — xref tables, indirect-object resolution, stream
decompression (FlateDecode, LZWDecode, ASCII85Decode), encryption — is
a large amount of mostly-uninteresting code. pdfcpu is mature, well-
tested, and gives us a parsed `*model.Context` to work with. We layer
the content-stream interpreter (which pdfcpu doesn't have) on top.

If pdfcpu's dependency footprint becomes a problem (it pulls in image
codecs we don't strictly need), the blast radius of swapping it out is
limited to `internal/pdf/reader.go`. The rest of the package is
stdlib-only.

## Roadmap

- `v0.0.x` — content-stream primitives.
- `v0.1.x` — text extraction: `Page.ExtractText`, `Page.Words`,
  `Page.ExtractTextSimple`.
- `v0.2.x` — table finding via ruling lines (this release):
  `Page.FindTables` / `Page.ExtractTables` covering the `lines` and
  `lines_strict` strategies.
- `v0.3.x` — remaining table strategies: `text` (word-alignment
  edges) and `explicit` (caller-supplied edges). Bundle the
  standard-14 AFM metrics so word bboxes (and therefore cell text)
  match pdfplumber to within 1 PDF point on standard fonts.
- `v0.4.x` — performance pass: parser benchmarking against pdfminer.six
  and pdfplumber on a representative document corpus.

## License

MIT. See [LICENSE](LICENSE).

## Acknowledgements

This library is a direct port of the algorithms in
[pdfminer.six](https://github.com/pdfminer/pdfminer.six) and
[pdfplumber](https://github.com/jsvine/pdfplumber). Their authors did
the hard work of figuring out how to robustly recover structure from
the PDF wire format; this is that work translated into Go.
