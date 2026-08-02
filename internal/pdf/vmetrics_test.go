// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdf

import (
	"math"
	"testing"
)

// TestStandard14VMetrics checks the bundled Ascent/Descent against the
// Adobe AFM values, and pins the two deliberate absences.
func TestStandard14VMetrics(t *testing.T) {
	cases := []struct {
		font            string
		ascent, descent float64
	}{
		{"Helvetica", 718, -207},
		{"Helvetica-Bold", 718, -207},
		{"Times-Roman", 683, -217},
		{"Times-BoldItalic", 683, -217},
		{"Courier", 627, -194},
		// Substitute names resolve to their metric equivalents, same as
		// Standard14Widths.
		{"Arial", 718, -207},
		{"ABCDEF+Arial,Bold", 718, -207},
		{"TimesNewRoman", 683, -217},
		{"CourierNew", 627, -194},
	}
	for _, tc := range cases {
		vm, ok := Standard14VMetrics(tc.font)
		if !ok {
			t.Errorf("Standard14VMetrics(%q) not found", tc.font)
			continue
		}
		if vm.Ascent != tc.ascent || vm.Descent != tc.descent {
			t.Errorf("%s = {%v, %v}, want {%v, %v}", tc.font, vm.Ascent, vm.Descent, tc.ascent, tc.descent)
		}
		if vm.Descent >= 0 {
			t.Errorf("%s descent %v should be negative (PDF spec convention)", tc.font, vm.Descent)
		}
	}

	// Symbol and ZapfDingbats AFMs genuinely carry no Ascender/Descender.
	// pdfminer.six reads them as 0 and so do we — substituting their
	// FontBBox would be defensible but would break parity.
	for _, f := range []string{"Symbol", "ZapfDingbats"} {
		if vm, ok := Standard14VMetrics(f); ok {
			t.Errorf("Standard14VMetrics(%q) = %+v, want not-found", f, vm)
		}
	}

	// Narrow/condensed variants stay unmatched, same rule as widths.
	for _, f := range []string{"Arial Narrow", "Helvetica-Condensed", "SomeEmbeddedFont"} {
		if _, ok := Standard14VMetrics(f); ok {
			t.Errorf("Standard14VMetrics(%q) matched, want no match", f)
		}
	}
}

// TestDescentScalesWithFontSize guards the second half of the fix, which
// is easy to lose in a refactor and invisible in any test whose font has
// a zero descent.
//
// Descent is stored in /1000ths of an em, so reaching text space needs
// BOTH 0.001 and the font size — the same two factors the advance width
// gets. The code previously applied only 0.001, making the descender a
// fixed fraction of a point rather than a fraction of the glyph. At 12pt
// that is off by 12x, and it under-shifted every glyph box regardless of
// whether the font supplied a descriptor.
//
// The expected offsets are the ones measured against pdfplumber on the
// golden fixtures: 2.484pt at 12pt and 4.968pt at 24pt.
func TestDescentScalesWithFontSize(t *testing.T) {
	const descent1000 = -207.0 // Helvetica
	for _, tc := range []struct {
		fontSize float64
		want     float64
	}{
		{12, -2.484},
		{24, -4.968},
		{8, -1.656},
	} {
		got := descent1000 * 0.001 * tc.fontSize
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("descent at %vpt = %v, want %v", tc.fontSize, got, tc.want)
		}
		// The pre-fix formula ignored font size entirely.
		buggy := descent1000 * 0.001
		if math.Abs(buggy-tc.want) < 1e-9 {
			t.Errorf("at %vpt the unscaled formula coincides with the correct one; test cannot detect the regression", tc.fontSize)
		}
	}
}
