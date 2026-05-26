// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdf

import "testing"

// TestEncodingByName checks that the four standard encodings produce
// the correct printable-ASCII mapping (identity over 0x20..0x7e) and
// that WinAnsi adds the smart-quote / dash / euro slots.
func TestEncodingByName(t *testing.T) {
	for _, name := range []string{"StandardEncoding", "WinAnsiEncoding", "MacRomanEncoding", "PDFDocEncoding"} {
		tab := EncodingByName(name)
		// Identity over printable ASCII.
		for c := byte(0x20); c < 0x7f; c++ {
			if tab[c] != string(rune(c)) {
				t.Errorf("%s[0x%02x] = %q, want %q", name, c, tab[c], string(rune(c)))
			}
		}
	}
	// WinAnsi-specific.
	wa := EncodingByName("WinAnsiEncoding")
	if wa[0x80] != "€" {
		t.Errorf("WinAnsi[0x80] = %q, want €", wa[0x80])
	}
	if wa[0x96] != "–" {
		t.Errorf("WinAnsi[0x96] = %q, want en-dash", wa[0x96])
	}
}

// TestApplyDifferences overlays a /Differences-style entry on
// StandardEncoding and asserts only the named slots changed.
func TestApplyDifferences(t *testing.T) {
	base := EncodingByName("StandardEncoding")
	diffs := []Difference{
		{CID: 39, GlyphName: "quotesingle"}, // ASCII apostrophe
		{CID: 96, GlyphName: "grave"},
	}
	out := ApplyDifferences(base, diffs)
	if out[39] != "'" {
		t.Errorf("out[39] = %q, want '", out[39])
	}
	if out[96] != "`" {
		t.Errorf("out[96] = %q, want `", out[96])
	}
	// Unaffected slot still identity.
	if out[65] != "A" {
		t.Errorf("out[65] = %q, want A", out[65])
	}
}

// TestAdobeGlyphRecognisers checks the uni-prefix and u-prefix
// recognisers since the table is small and most real fonts emit
// glyphs via these forms.
func TestAdobeGlyphRecognisers(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"A", "A"},
		{"comma", ","},
		{"fi", "fi"},
		{"uni0041", "A"},
		{"uni004100420043", "ABC"},
		{"u0041", "A"},
		{"u1F600", "😀"},
		{"notaknownname", ""},
	}
	for _, tc := range cases {
		got := AdobeGlyphToUnicode(tc.in)
		if got != tc.want {
			t.Errorf("AdobeGlyphToUnicode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestFontDecodeUnicodeFallback verifies the lookup priority:
// ToUnicode > encoding > Latin-1 > (cid:NNN).
func TestFontDecodeUnicodeFallback(t *testing.T) {
	enc := EncodingByName("WinAnsiEncoding")
	f := &Font{
		BaseFont:           "TestFont",
		IsSimple:           true,
		cid2unicodeEncoding: enc,
	}
	// Encoding hit.
	if got := f.DecodeUnicode(0x41); got != "A" {
		t.Errorf("DecodeUnicode(0x41) = %q, want A", got)
	}
	// Latin-1 fallback for an unmapped printable byte.
	f.cid2unicodeEncoding[0x42] = "" // clear B
	if got := f.DecodeUnicode(0x42); got != "B" {
		t.Errorf("DecodeUnicode(0x42) fallback = %q, want B", got)
	}
	// Out of range: (cid:NNN).
	if got := f.DecodeUnicode(0x100); got != "(cid:256)" {
		t.Errorf("DecodeUnicode(0x100) = %q, want (cid:256)", got)
	}

	// ToUnicode takes priority over the encoding table.
	tu := NewCMap()
	tu.cid2unicode[0x41] = "Z"
	f.ToUnicode = tu
	if got := f.DecodeUnicode(0x41); got != "Z" {
		t.Errorf("DecodeUnicode(0x41) with ToUnicode = %q, want Z", got)
	}
}
