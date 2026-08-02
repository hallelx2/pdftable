// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdf

import "testing"

// TestStandard14WidthsKnownGlyphs checks a handful of AFM values against
// the actual Adobe Core 14 metrics (space/i/m/W widths differ enough
// between Helvetica, Times, and Courier that a transcription error would
// show up immediately as a wrong test value here).
func TestStandard14WidthsKnownGlyphs(t *testing.T) {
	cases := []struct {
		font string
		r    rune
		want float64
	}{
		{"Helvetica", ' ', 278},
		{"Helvetica", 'i', 222},
		{"Helvetica", 'm', 833},
		{"Helvetica", 'W', 944},
		// Courier is monospace: every glyph is 600/1000, unlike Helvetica
		// and Times where widths vary per glyph.
		{"Courier", ' ', 600},
		{"Courier", 'm', 600},
		{"Courier", 'i', 600},
		{"Times-Roman", ' ', 250},
		{"Times-Roman", 'i', 278},
		{"Times-Roman", 'm', 778},
	}
	for _, tc := range cases {
		w, ok := Standard14Widths(tc.font)
		if !ok {
			t.Fatalf("Standard14Widths(%q) not found", tc.font)
		}
		if got := w[tc.r]; got != tc.want {
			t.Errorf("%s width of %q = %v, want %v", tc.font, tc.r, got, tc.want)
		}
	}
}

// TestStandard14Aliases checks that common non-Adobe substitute names
// (the TrueType fonts real-world PDFs actually reference) resolve to
// the same metrics as their canonical Adobe counterpart, and that a
// subset tag ("ABCDEF+Arial,Bold") doesn't block the match.
func TestStandard14Aliases(t *testing.T) {
	base, ok := Standard14Widths("Helvetica-Bold")
	if !ok {
		t.Fatal("Helvetica-Bold not found")
	}
	aliases := []string{
		"Arial,Bold",
		"Arial-Bold",
		"Arial-BoldMT",
		"ABCDEF+Arial,Bold", // subset-tagged
	}
	for _, alias := range aliases {
		w, ok := Standard14Widths(alias)
		if !ok {
			t.Errorf("Standard14Widths(%q) not found, want alias to Helvetica-Bold", alias)
			continue
		}
		if w['A'] != base['A'] {
			t.Errorf("%s width of 'A' = %v, want %v (Helvetica-Bold)", alias, w['A'], base['A'])
		}
	}

	// Case- and whitespace-insensitive.
	if _, ok := Standard14Widths("ARIAL"); !ok {
		t.Error(`Standard14Widths("ARIAL") not found`)
	}
	if w, ok := Standard14Widths("TimesNewRoman,Bold"); !ok || w['A'] == 0 {
		t.Error(`Standard14Widths("TimesNewRoman,Bold") should resolve to Times-Bold`)
	}
}

// TestStandard14RejectsNarrowVariants documents the deliberate design
// choice: fonts whose name merely CONTAINS a standard-14 family name but
// names a metrically-different variant (Narrow/Condensed) must NOT match,
// because substituting regular-width metrics for a condensed font would
// silently produce a plausible-but-wrong bbox -- worse than the honest
// flat-500 fallback CharWidth uses for anything unrecognized.
func TestStandard14RejectsNarrowVariants(t *testing.T) {
	rejected := []string{
		"Arial Narrow",
		"ArialNarrow",
		"Helvetica-Narrow",
		"Helvetica-Condensed",
		"Times New Roman PS Condensed",
		"SomeRandomEmbeddedFont",
	}
	for _, name := range rejected {
		if _, ok := Standard14Widths(name); ok {
			t.Errorf("Standard14Widths(%q) matched, want no match (narrow/condensed/unknown fonts must stay unmatched)", name)
		}
	}
}

// TestCharWidthStandard14Fallback is the end-to-end check: a Font built
// the way readFont constructs one for a standard font with no /Widths
// array (Standard14 set, Widths empty) must report real per-glyph AFM
// widths through CharWidth, must still prefer an explicit /Widths entry
// when one exists, and must fall back to the flat 500 guess when the
// font is not a Standard14 match -- exactly the current behaviour for
// every other font, unregressed.
func TestCharWidthStandard14Fallback(t *testing.T) {
	std14, ok := Standard14Widths("Helvetica")
	if !ok {
		t.Fatal("Standard14Widths(Helvetica) not found")
	}

	f := &Font{
		BaseFont:            "Helvetica",
		IsSimple:            true,
		cid2unicodeEncoding: EncodingByName("WinAnsiEncoding"),
		Widths:              map[uint16]float64{},
		Standard14:          std14,
	}

	// 'i' (0x69) and 'm' (0x6d) must resolve to their real, DIFFERENT
	// AFM widths -- not the flat 500 guess a nil Standard14 would give.
	iWidth := f.CharWidth(0x69)
	mWidth := f.CharWidth(0x6d)
	if iWidth != 222 {
		t.Errorf("CharWidth('i') = %v, want 222 (Helvetica AFM)", iWidth)
	}
	if mWidth != 833 {
		t.Errorf("CharWidth('m') = %v, want 833 (Helvetica AFM)", mWidth)
	}
	if iWidth == mWidth {
		t.Error("CharWidth('i') and CharWidth('m') are equal -- Standard14 fallback isn't being consulted (would indicate a regression to the flat-500 guess)")
	}

	// An explicit /Widths entry, when present, still wins over Standard14.
	f.Widths[0x69] = 999
	if got := f.CharWidth(0x69); got != 999 {
		t.Errorf("CharWidth('i') with explicit Widths override = %v, want 999", got)
	}
	delete(f.Widths, 0x69)

	// A font with no Standard14 table (the common case: embedded font,
	// or a name that doesn't resolve) is unchanged: flat 500 for any
	// CID with no explicit width and no DefaultWidth.
	plain := &Font{
		BaseFont: "SomeEmbeddedSubsetFont",
		IsSimple: true,
		Widths:   map[uint16]float64{},
	}
	if got := plain.CharWidth(0x69); got != 500 {
		t.Errorf("CharWidth on a non-Standard14 font = %v, want 500 (unchanged fallback)", got)
	}
}
