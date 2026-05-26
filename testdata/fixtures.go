// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

// Package testdata builds the test PDFs used by pdftable's tests.
//
// We hand-craft the PDFs as byte slices rather than checking in
// binaries because:
//
//  1. It makes the test fixtures readable in the repo — every glyph,
//     position, and operator is right there in the source.
//  2. It avoids storing PDF binaries in git history (small win on
//     repo size, larger win on diff legibility).
//  3. Tests can mutate a fixture to probe edge cases without checking
//     in a separate file for each variation.
//
// The PDFs are intentionally minimal: one page each, single font
// (Helvetica from the standard 14), no compression, no encryption.
// We rely on the parser to dereference indirect objects properly so
// the structure is realistic, but we don't exercise pdfcpu's stream
// decompression here (it's exercised separately by real-world PDFs
// the engine ingests).
package testdata

import (
	"fmt"
	"strings"
)

// Hello returns a minimal PDF with one page containing the text
// "Hello, world!" at position (72, 720) with Helvetica 12pt.
//
// Construction follows the PDF 1.7 grammar precisely:
//
//   - The %PDF-1.4 header signals the version.
//   - Objects 1-5 form the standard root → pages → page → font chain.
//   - Object 6 is the page's content stream: a `BT ... ET` text
//     object with Helvetica selected via `/F1 12 Tf` and the string
//     drawn via `Tj`.
//   - The xref table is constructed by recording each object's byte
//     offset as we build the body, so the final %%EOF section is
//     byte-accurate.
//
// The function is deterministic — same input → same bytes — which
// matters because the test assertions use absolute byte offsets in
// some places.
func Hello() []byte {
	return BuildSinglePage(`BT
/F1 12 Tf
72 720 Td
(Hello, world!) Tj
ET
`)
}

// Rules returns a minimal PDF whose content stream draws four lines
// (two horizontal, two vertical) and one rectangle. We use simple
// coordinates: a 100x100 box with one stroked diagonal line and one
// stroked horizontal line outside the box.
func Rules() []byte {
	// PDF content stream operators:
	//   re   = rectangle (x y w h)
	//   m    = moveto (x y)
	//   l    = lineto (x y)
	//   S    = stroke
	//   1 w  = set line width to 1pt
	return BuildSinglePage(`1 w
% A stroked rectangle, 100x100 at (100, 600).
100 600 100 100 re
S
% A horizontal line below, 50 pt long.
100 580 m
150 580 l
S
% A vertical line below, 50 pt long.
100 500 m
100 550 l
S
`)
}

// BuildSinglePage assembles a single-page PDF whose content stream is
// the provided literal text. The page is US-letter (612 x 792 pt),
// uses Helvetica from the standard 14 (so we don't need to embed a
// font program), and has no compression on the content stream.
//
// The xref table is generated correctly: we accumulate each object's
// byte offset as we write the body, then emit the xref subsection at
// the end with one entry per object. The trailer points to /Root
// (object 1) and /Size = number_of_objects + 1 (entry 0 is the
// "head of free list" sentinel).
//
// Output bytes are stable across runs.
func BuildSinglePage(content string) []byte {
	const header = "%PDF-1.4\n%\xe2\xe3\xcf\xd3\n"

	// Object bodies. Numbered 1..6.
	objects := []string{
		// 1: Catalog.
		`<< /Type /Catalog /Pages 2 0 R >>`,
		// 2: Pages tree root.
		`<< /Type /Pages /Kids [3 0 R] /Count 1 >>`,
		// 3: Page.
		`<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> /ProcSet [/PDF /Text] >> /Contents 5 0 R >>`,
		// 4: Font (Helvetica from the standard 14; no descriptor needed).
		`<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>`,
		// 5: Content stream. Length is computed below.
		"", // placeholder — filled in after we know content length.
	}

	// Build the content-stream object with its /Length set correctly.
	streamBody := []byte(content)
	objects[4] = fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(streamBody), streamBody)

	// Assemble the body, recording byte offsets for xref.
	var b strings.Builder
	b.WriteString(header)

	offsets := make([]int, len(objects))
	for i, body := range objects {
		offsets[i] = b.Len()
		b.WriteString(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", i+1, body))
	}

	// xref table.
	xrefPos := b.Len()
	b.WriteString("xref\n")
	b.WriteString(fmt.Sprintf("0 %d\n", len(objects)+1))
	// Object 0 is the head of the free list.
	b.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		b.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
	}

	// Trailer.
	b.WriteString("trailer\n")
	b.WriteString(fmt.Sprintf("<< /Size %d /Root 1 0 R >>\n", len(objects)+1))
	b.WriteString("startxref\n")
	b.WriteString(fmt.Sprintf("%d\n", xrefPos))
	b.WriteString("%%EOF\n")

	return []byte(b.String())
}
