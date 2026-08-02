// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdf

// VMetrics holds a font's vertical extrema in /1000ths of an em.
type VMetrics struct {
	Ascent  float64
	Descent float64 // negative, per the PDF spec convention
}

// afmVMetrics is the Ascent/Descent pair for each of the 14 standard
// fonts, from the same Adobe AFM data as afmGlyphWidths.
//
// The 14 standard fonts may omit /FontDescriptor for exactly the same
// reason they may omit /Widths (PDF 1.7 SS9.6.2.2): a consumer is
// expected to already know their metrics. Without these values
// Font.Descent stays 0 and a glyph's box degenerates to
// [baseline, baseline+size], sitting descent*size too high.
//
// Symbol and ZapfDingbats are deliberately absent: their AFM files
// genuinely carry no Ascender/Descender entry, and pdfminer.six -- the
// parity target -- reads them as 0. Substituting their FontBBox would
// be defensible but would diverge from the reference implementation.
var afmVMetrics = map[string]VMetrics{
	"Helvetica":             {Ascent: 718, Descent: -207},
	"Helvetica-Bold":        {Ascent: 718, Descent: -207},
	"Helvetica-Oblique":     {Ascent: 718, Descent: -207},
	"Helvetica-BoldOblique": {Ascent: 718, Descent: -207},
	"Times-Roman":           {Ascent: 683, Descent: -217},
	"Times-Bold":            {Ascent: 683, Descent: -217},
	"Times-Italic":          {Ascent: 683, Descent: -217},
	"Times-BoldItalic":      {Ascent: 683, Descent: -217},
	"Courier":               {Ascent: 627, Descent: -194},
	"Courier-Bold":          {Ascent: 627, Descent: -194},
	"Courier-Oblique":       {Ascent: 627, Descent: -194},
	"Courier-BoldOblique":   {Ascent: 627, Descent: -194},
}

// Standard14VMetrics returns the AFM vertical metrics for baseFont if it
// names one of the 14 standard fonts, resolving the same aliases and
// subset tags as Standard14Widths. ok is false for Symbol and
// ZapfDingbats, whose AFMs carry no such values.
func Standard14VMetrics(baseFont string) (vm VMetrics, ok bool) {
	canonical, found := standard14Aliases[normalizeStandard14Key(baseFont)]
	if !found {
		return VMetrics{}, false
	}
	vm, ok = afmVMetrics[canonical]
	return vm, ok
}
