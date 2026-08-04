# 4 · Building pdftable

*Part four of seven on building [pdftable](https://github.com/hallelx2/pdftable).*

Vectorless is written in Go. The best PDF table extractor in existence is [pdfplumber](https://github.com/jsvine/pdfplumber), and it is written in Python.

Go's PDF ecosystem has libraries that render pages to images, libraries that edit metadata, and libraries that extract bag-of-words text. None of them give you what pdfplumber gives Python users: a per-page object model of positioned primitives that you can run layout heuristics over. That gap is the reason pdftable exists.

## A port, not a reimagining

The decision that shaped everything: **match pdfplumber's algorithms exactly rather than invent better ones.**

That is a less exciting choice than it sounds, and it was the right one. pdfplumber's heuristics encode a decade of contact with real, malformed PDFs. Its word-grouping tolerances, its cell-selection order, its treatment of zero-area ruling overlaps — those constants exist because somebody hit a document that needed them. Re-deriving that from first principles would mean rediscovering the same edge cases one production incident at a time.

Parity is also testable in a way that "better" is not. If pdfplumber and pdftable disagree on a fixture, exactly one of us is wrong and the difference is a bug to explain. Without a reference there is no signal at all — which turns out to be the theme of part six.

So the port is faithful down to details that look like mistakes:

- Word grouping clusters on Y1 (the visual top of a glyph) and sorts by X0, matching `WordExtractor`.
- Cell selection walks nearest-first outward, below-outer then right-inner, first-close-wins, one cell per anchor — byte-for-byte with `TableFinder`.
- `Intersect` treats touching-but-not-overlapping boxes as non-overlapping, matching pdfplumber's `o_height + o_width > 0` check, so a ruler grazing a word's box is not reported as intersecting it.

Where the API differs it is because Go differs. pdfplumber returns dicts keyed `"x0"`, `"top"`, `"x1"`, `"bottom"` in image space. pdftable returns a struct with `X0, Y0, X1, Y1` in PDF user space. Both are defensible; mixing them silently is not, so the two coordinate systems have separate types (`BBox` and `ViewRect`) that cannot be passed to each other by accident.

## The layers

```
pdftable/              the public API — Document, Page, Char, Word, Table, BBox
  internal/pdf/        content-stream interpreter, fonts, encodings, CMaps
  internal/layout/     edges, intersections, cells, clustering
  cmd/pdftable/        CLI
```

The interpreter is a straightforward operator dispatch loop over the content stream, maintaining graphics state and text state, emitting typed events to a sink. Unknown operators drop their operands and continue — PDFs in the wild carry private extensions, and aborting a page because of one is worse than ignoring it.

Only `internal/pdf/reader.go` touches pdfcpu. Everything else is standard-library-only. If pdfcpu ever needs to go — it is heavy and pulls in image codecs pdftable never uses — one file changes.

## The API

```go
doc, _ := pdftable.OpenFile("report.pdf")
defer doc.Close()

for n, page := range doc.Pages() {          // Go 1.23 range-over-func
    words, _  := page.Words(pdftable.DefaultWordOpts())
    text, _   := page.ExtractText(pdftable.DefaultTextOpts())
    tables, _ := page.ExtractTables(pdftable.DefaultTableSettings())
}
```

Four strategies, selectable per axis independently, so `vertical="text"` with `horizontal="lines"` works — which turns out to matter for the booktabs case from part three.

Every returned object carries geometry. `Table.CellsBBox[i][j]` is the box of `Table.Rows[i][j]`, which is what makes cell-level citations possible.

## Two design calls worth defending

**Alias matching is exact, not fuzzy.** Real PDFs reference `Arial` when they mean Helvetica, and `TimesNewRoman` when they mean Times. Those are true 1:1 metric substitutes, so they alias in, along with subset tags (`ABCDEF+Arial`) and case variants.

It was tempting to match any BaseFont *containing* "Helvetica". I rejected that: `Arial Narrow` and `Helvetica-Condensed` share the family name but not the metrics. Substituting regular widths there produces a bounding box that looks plausible and is wrong — worse than the honest flat-500 fallback, which at least fails visibly. A test pins the narrow variants as unmatched so nobody "improves" it later.

**Settings that break parity are opt-in.** `MergeSplitTokens` rejoins values a column boundary cut in half — turning `| ( | 16,135) |` back into `| (16,135) |`. Useful when feeding a table to a language model. It defaults to off, because pdfplumber produces the same splits and enabling it silently would break the compatibility the library promises. Part six has the measurement that makes this more than a preference.

## Where the tests are

Golden files generated from pdfplumber, diffed automatically. Fixtures for cases where pdfplumber is *wrong* — Symbol, ZapfDingbats — asserted directly instead, because generating a golden there would pin the wrong answer.

That distinction took an embarrassing while to arrive at, and part five is about why.

---

**Next:** [The bugs that never crashed](05-the-bugs-that-never-crashed.md) — five silent corruptions, including eight thousandths of a point that flipped the sign of a number in an annual report.
