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

`v0.0.1` — content-stream primitives layer. The public API surface is
stable; higher-level operations (`ExtractText`, `FindTables`,
`ExtractTables`) are coming in subsequent releases.

[![Go Reference](https://pkg.go.dev/badge/github.com/hallelx2/pdftable.svg)](https://pkg.go.dev/github.com/hallelx2/pdftable)
[![CI](https://github.com/hallelx2/pdftable/actions/workflows/test.yml/badge.svg)](https://github.com/hallelx2/pdftable/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Install

```sh
go get github.com/hallelx2/pdftable@v0.0.1
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
        chars, _ := page.Chars()
        rects, _ := page.Rects()
        lines, _ := page.Lines()
        fmt.Printf("page %d: %d chars, %d rects, %d lines\n",
            n, len(chars), len(rects), len(lines))

        // Each Char carries its own bbox, font name, font size, and
        // upright flag — feed them to your own layout algorithm.
        for _, c := range chars[:min(5, len(chars))] {
            fmt.Printf("  %q at (%.1f, %.1f) - (%.1f, %.1f) %s %.1fpt\n",
                c.Text, c.X0, c.Y0, c.X1, c.Y1, c.FontName, c.FontSize)
        }
    }
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

// Sentinel errors.
var (
    ErrInvalidPDF     = errors.New("pdftable: invalid PDF")
    ErrPageOutOfRange = errors.New("pdftable: page out of range")
    ErrUnsupported    = errors.New("pdftable: unsupported feature")
    ErrEncrypted      = errors.New("pdftable: encrypted PDF (decryption not yet supported)")
)
```

## Side-by-side comparison with pdfplumber

```python
# Python (pdfplumber)
import pdfplumber

with pdfplumber.open("report.pdf") as pdf:
    page = pdf.pages[0]
    for char in page.chars:
        print(char["text"], char["x0"], char["y0"])
```

```go
// Go (pdftable)
import "github.com/hallelx2/pdftable"

doc, _ := pdftable.OpenFile("report.pdf")
defer doc.Close()
page, _ := doc.Page(1)
chars, _ := page.Chars()
for _, c := range chars {
    fmt.Println(c.Text, c.X0, c.Y0)
}
```

Three differences worth noting:

1. **Page indexing is 1-based**, matching the PDF spec and pdfplumber's
   `pdf.pages[0]` is actually the first page (Python is 0-indexed,
   pdfplumber compensates). Our `Page(1)` is the same first page.
2. **Coordinates are in PDF user space with origin at bottom-left**.
   pdfplumber by default reports `top` (origin top-left, Y growing down)
   on its chars; we report `Y0` / `Y1` in PDF native coordinates. The
   conversion is `top = mediabox.height - Y1`.
3. **No layout-analysis methods yet**. `extract_text`, `extract_tables`,
   `find_tables` are coming in later releases.

## Architecture

```
pdftable/
├── pdftable.go        // Open / OpenBytes / OpenFile entry points
├── pdf.go             // Document interface + implementation
├── page.go            // Page interface + implementation
├── char.go            // Public Char / Line / Rect / Curve / Objects
├── errors.go          // Sentinel errors
└── internal/pdf/
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

- `v0.0.x` — content-stream primitives (this release).
- `v0.1.x` — text extraction: `Page.ExtractText`, `Page.Words`, word
  grouping with reading-order sort.
- `v0.2.x` — table finding: `Page.FindTables` using ruling-line +
  whitespace heuristics, `Page.ExtractTables` returning row/cell text.
- `v0.3.x` — performance pass: parser benchmarking against pdfminer.six
  and pdfplumber on a representative document corpus.

## License

MIT. See [LICENSE](LICENSE).

## Acknowledgements

This library is a direct port of the algorithms in
[pdfminer.six](https://github.com/pdfminer/pdfminer.six) and
[pdfplumber](https://github.com/jsvine/pdfplumber). Their authors did
the hard work of figuring out how to robustly recover structure from
the PDF wire format; this is that work translated into Go.
