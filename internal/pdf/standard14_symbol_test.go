// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdf

import "testing"

// TestStandard14GlyphCoverage is the regression guard for the whole
// name-to-Unicode layer: every glyph in every one of the 14 standard
// fonts must resolve to exactly one rune, with nothing silently dropped.
//
// This test exists because the AFM tables originally shipped with no
// coverage assertion, and 145 of Symbol's 190 glyphs plus 201 of
// ZapfDingbats' 202 were being discarded at init() -- the widths were
// bundled but unreachable, and every one of those glyphs quietly fell
// back to the flat 500 guess. Nothing failed; the data was just dead.
// Asserting exact counts means that cannot recur unnoticed.
func TestStandard14GlyphCoverage(t *testing.T) {
	want := map[string]int{
		"Helvetica": 229, "Helvetica-Bold": 229,
		"Helvetica-Oblique": 229, "Helvetica-BoldOblique": 229,
		"Times-Roman": 229, "Times-Bold": 229,
		"Times-Italic": 229, "Times-BoldItalic": 229,
		"Courier": 229, "Courier-Bold": 229,
		"Courier-Oblique": 229, "Courier-BoldOblique": 229,
		"Symbol": 190, "ZapfDingbats": 202,
	}
	if len(afmGlyphWidths) != len(want) {
		t.Fatalf("afmGlyphWidths has %d fonts, want %d", len(afmGlyphWidths), len(want))
	}
	for font, wantN := range want {
		byName, ok := afmGlyphWidths[font]
		if !ok {
			t.Errorf("%s missing from afmGlyphWidths", font)
			continue
		}
		if len(byName) != wantN {
			t.Errorf("%s has %d AFM glyphs, want %d", font, len(byName), wantN)
		}
		// Every name resolves...
		for gname := range byName {
			if u := standard14GlyphToUnicode(font, gname); u == "" {
				t.Errorf("%s: glyph %q does not resolve to Unicode", font, gname)
			}
		}
		// ...and no two names collapse onto the same rune, which would
		// silently drop a width from the rune-keyed table.
		if got := len(standard14WidthsByUnicode[font]); got != wantN {
			t.Errorf("%s resolved to %d distinct runes, want %d (a shortfall means two glyph names collided on one rune)", font, got, wantN)
		}
	}
}

// TestZapfDingbatsNamesStayFontScoped pins the deliberate split between
// the two new tables. Symbol's names are real AGL entries and belong in
// the global resolver; ZapfDingbats' "aNN" names are font-specific and
// must not be, or a Latin font whose /Differences array names "a1" would
// silently decode as U+2701 SCISSORS instead of its own glyph.
func TestZapfDingbatsNamesStayFontScoped(t *testing.T) {
	for _, name := range []string{"a1", "a10", "a100", "a206"} {
		if u := AdobeGlyphToUnicode(name); u != "" {
			t.Errorf("AdobeGlyphToUnicode(%q) = %q, want \"\" (dingbat names must not resolve globally)", name, u)
		}
		if u := standard14GlyphToUnicode("ZapfDingbats", name); u == "" {
			t.Errorf("standard14GlyphToUnicode(ZapfDingbats, %q) = \"\", want a dingbat rune", name)
		}
	}
	// A name absent from Adobe's zapfdingbats.txt still falls through to
	// the shared AGL resolver -- "space" is the one that really occurs.
	if u := standard14GlyphToUnicode("ZapfDingbats", "space"); u != " " {
		t.Errorf("standard14GlyphToUnicode(ZapfDingbats, \"space\") = %q, want \" \"", u)
	}
	if got := standard14WidthsByUnicode["ZapfDingbats"]['✁']; got != 974 {
		t.Errorf("ZapfDingbats width of a1 (U+2701) = %v, want 974", got)
	}
}

// TestSymbolGlyphNamesResolveGlobally checks the Symbol additions, and
// deliberately pins the two AGL values that look like typos but are not.
func TestSymbolGlyphNamesResolveGlobally(t *testing.T) {
	cases := []struct{ name, want string }{
		{"Alpha", "Α"},
		{"alpha", "α"},
		{"universal", "∀"},
		{"club", "♣"},
		{"summation", "∑"},
		{"weierstrass", "℘"},
		// Adobe's glyphlist.txt really does map these to the maths
		// codepoints rather than the Greek ones. Matching it is what
		// keeps us byte-compatible with pdfminer.six.
		{"Delta", "∆"}, // INCREMENT, not U+0394
		{"Omega", "Ω"}, // OHM SIGN, not U+03A9
	}
	for _, tc := range cases {
		if got := AdobeGlyphToUnicode(tc.name); got != tc.want {
			t.Errorf("AdobeGlyphToUnicode(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}

	// adobeGlyphTable is consulted first, so names present in both tables
	// keep their original values — no behaviour change for them.
	if got := AdobeGlyphToUnicode("mu"); got != "µ" {
		t.Errorf(`AdobeGlyphToUnicode("mu") = %q, want "µ" (adobeGlyphTable must win)`, got)
	}
	if got := AdobeGlyphToUnicode("multiply"); got != "×" {
		t.Errorf(`AdobeGlyphToUnicode("multiply") = %q, want "×" (adobeGlyphTable must win)`, got)
	}
}

// TestSymbolBuiltinEncoding covers the second half of the fix. Symbol
// and ZapfDingbats ship their own encoding, so a font dict that declares
// no /Encoding must not be handed StandardEncoding: code 0x61 in Symbol
// is alpha, not "a".
func TestSymbolBuiltinEncoding(t *testing.T) {
	enc, ok := Standard14BuiltinEncoding("Symbol")
	if !ok {
		t.Fatal("Standard14BuiltinEncoding(Symbol) not found")
	}
	if enc[0x61] != "α" {
		t.Errorf("Symbol 0x61 = %q, want α — this is the StandardEncoding mis-decode the fix targets", enc[0x61])
	}
	if enc[0x44] != "∆" {
		t.Errorf("Symbol 0x44 = %q, want ∆ (Delta)", enc[0x44])
	}

	zenc, ok := Standard14BuiltinEncoding("ZapfDingbats")
	if !ok {
		t.Fatal("Standard14BuiltinEncoding(ZapfDingbats) not found")
	}
	if zenc[0x61] == "a" {
		t.Error("ZapfDingbats 0x61 decoded as \"a\" — built-in encoding not applied")
	}

	// The other 12 standard fonts have no built-in encoding and must keep
	// using the four PDF base encodings.
	for _, f := range []string{"Helvetica", "Times-Roman", "Courier", "Arial"} {
		if _, ok := Standard14BuiltinEncoding(f); ok {
			t.Errorf("Standard14BuiltinEncoding(%q) returned a built-in encoding, want none", f)
		}
	}
}

// TestZapfDingbatsDifferencesUseFontScopedResolver covers the failure
// mode that keeping "aNN" out of the global resolver introduced.
//
// ApplyDifferences resolves every name through AdobeGlyphToUnicode,
// which cannot see zapfDingbatsGlyphTable. So a ZapfDingbats font whose
// /Encoding dict carries only /Differences kept the right built-in base
// encoding and then had those very slots overwritten with "" -- the
// glyph decoded as "(cid:N)" and lost its width, even though we hold the
// mapping. readFont now passes a font-scoped resolver instead.
func TestZapfDingbatsDifferencesUseFontScopedResolver(t *testing.T) {
	base, ok := Standard14BuiltinEncoding("ZapfDingbats")
	if !ok {
		t.Fatal("ZapfDingbats built-in encoding not found")
	}
	diffs := []Difference{{CID: 0x41, GlyphName: "a1"}}

	// The global resolver blanks the slot — the bug.
	if got := ApplyDifferences(base, diffs)[0x41]; got != "" {
		t.Errorf("ApplyDifferences resolved %q to %q; test premise is stale", "a1", got)
	}

	// The font-scoped resolver keeps it.
	resolve, ok := Standard14GlyphResolver("ZapfDingbats")
	if !ok {
		t.Fatal("Standard14GlyphResolver(ZapfDingbats) not found")
	}
	enc := ApplyDifferencesWith(base, diffs, resolve)
	if enc[0x41] != "✁" {
		t.Errorf("ApplyDifferencesWith slot 0x41 = %q, want ✁ (U+2701, the a1 dingbat)", enc[0x41])
	}

	// And the width now resolves rather than falling back to 500.
	f := &Font{
		BaseFont:            "ZapfDingbats",
		IsSimple:            true,
		cid2unicodeEncoding: enc,
		Widths:              map[uint16]float64{},
	}
	f.Standard14, _ = Standard14Widths("ZapfDingbats")
	if got := f.CharWidth(0x41); got != 974 {
		t.Errorf("CharWidth(0x41) = %v, want 974 (a1)", got)
	}

	// Non-standard-14 fonts keep the global resolver.
	if _, ok := Standard14GlyphResolver("SomeEmbeddedSubsetFont"); ok {
		t.Error("Standard14GlyphResolver matched a non-standard-14 font")
	}
}

// TestSymbolCharWidthEndToEnd is the payoff: a Symbol font with no
// /Widths array, built the way readFont now builds one, must report the
// real per-glyph AFM widths rather than the flat 500 guess.
func TestSymbolCharWidthEndToEnd(t *testing.T) {
	widths, ok := Standard14Widths("Symbol")
	if !ok {
		t.Fatal("Standard14Widths(Symbol) not found")
	}
	enc, ok := Standard14BuiltinEncoding("Symbol")
	if !ok {
		t.Fatal("Standard14BuiltinEncoding(Symbol) not found")
	}
	f := &Font{
		BaseFont:            "Symbol",
		IsSimple:            true,
		cid2unicodeEncoding: enc,
		Widths:              map[uint16]float64{},
		Standard14:          widths,
	}

	// Extracted text is Greek, not mis-mapped Latin.
	if got := f.DecodeUnicode(0x61); got != "α" {
		t.Errorf("DecodeUnicode(0x61) = %q, want α", got)
	}
	// alpha=631, Alpha=722, universal=713 — all distinct, and none is 500.
	for _, tc := range []struct {
		cid  uint16
		want float64
		name string
	}{
		{0x61, 631, "alpha"},
		{0x41, 722, "Alpha"},
		{0x22, 713, "universal"},
	} {
		if got := f.CharWidth(tc.cid); got != tc.want {
			t.Errorf("CharWidth(%#x) [%s] = %v, want %v", tc.cid, tc.name, got, tc.want)
		}
	}
}
