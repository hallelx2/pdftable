// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdf

import (
	"strings"
	"unicode/utf8"
)

// standard14Aliases maps a normalized PDF BaseFont name to one of the 14
// canonical standard-font names used as keys into afmGlyphWidths.
// Normalization (see normalizeStandard14Key) strips any subset tag,
// lowercases, and removes internal whitespace, so "ABCDEF+Arial,Bold",
// "Arial,Bold", and "arial, bold" all resolve to the same key.
//
// This is an EXACT-match table on purpose, not a substring/fuzzy
// heuristic. Real-world PDFs commonly reference the standard 14 fonts
// by their non-Adobe TrueType equivalents (Arial for Helvetica,
// Times New Roman for Times, Courier New for Courier) -- those are
// safe 1:1 metric substitutes and worth aliasing. But similarly-named
// families with genuinely different metrics -- Arial Narrow, Helvetica
// Condensed, Times New Roman PS Condensed, any "Narrow"/"Condensed"
// variant -- are deliberately left OUT. Silently substituting regular-
// width metrics for a condensed font produces a plausible-looking but
// wrong bbox, which is worse for downstream table/layout code than the
// honest flat-500 fallback Font.CharWidth already applies to anything
// not recognized here. When in doubt, this table stays conservative.
var standard14Aliases = map[string]string{
	"helvetica":             "Helvetica",
	"helvetica,bold":        "Helvetica-Bold",
	"helvetica-bold":        "Helvetica-Bold",
	"helvetica,italic":      "Helvetica-Oblique",
	"helvetica,oblique":     "Helvetica-Oblique",
	"helvetica-oblique":     "Helvetica-Oblique",
	"helvetica,bolditalic":  "Helvetica-BoldOblique",
	"helvetica,boldoblique": "Helvetica-BoldOblique",
	"helvetica-boldoblique": "Helvetica-BoldOblique",

	"times-roman":       "Times-Roman",
	"timesnewroman":     "Times-Roman",
	"timesnewromanpsmt": "Times-Roman",

	"times-bold":             "Times-Bold",
	"timesnewroman,bold":     "Times-Bold",
	"timesnewromanps-boldmt": "Times-Bold",

	"times-italic":             "Times-Italic",
	"timesnewroman,italic":     "Times-Italic",
	"timesnewromanps-italicmt": "Times-Italic",

	"times-bolditalic":             "Times-BoldItalic",
	"timesnewroman,bolditalic":     "Times-BoldItalic",
	"timesnewromanps-bolditalicmt": "Times-BoldItalic",

	"courier":        "Courier",
	"couriernew":     "Courier",
	"couriernewpsmt": "Courier",

	"courier-bold":        "Courier-Bold",
	"couriernew,bold":     "Courier-Bold",
	"couriernewps-boldmt": "Courier-Bold",

	"courier-oblique":       "Courier-Oblique",
	"courier,italic":        "Courier-Oblique",
	"couriernew,italic":     "Courier-Oblique",
	"couriernewps-italicmt": "Courier-Oblique",

	"courier-boldoblique":       "Courier-BoldOblique",
	"courier,bolditalic":        "Courier-BoldOblique",
	"couriernew,bolditalic":     "Courier-BoldOblique",
	"couriernewps-bolditalicmt": "Courier-BoldOblique",

	"arial":              "Helvetica",
	"arialmt":            "Helvetica",
	"arial,bold":         "Helvetica-Bold",
	"arial-bold":         "Helvetica-Bold",
	"arial-boldmt":       "Helvetica-Bold",
	"arial,italic":       "Helvetica-Oblique",
	"arial-italic":       "Helvetica-Oblique",
	"arial-italicmt":     "Helvetica-Oblique",
	"arial,bolditalic":   "Helvetica-BoldOblique",
	"arial-bolditalic":   "Helvetica-BoldOblique",
	"arial-bolditalicmt": "Helvetica-BoldOblique",

	"symbol":       "Symbol",
	"zapfdingbats": "ZapfDingbats",
}

// stripSubsetTag removes a PDF subset prefix ("ABCDEF+") from name, per
// PDF 1.7 §9.6.4: exactly six uppercase ASCII letters followed by '+'.
// Subsetted embeddings of a standard font ARE possible (a producer can
// embed a renamed, subsetted copy of Helvetica) and still omit /Widths
// if the embedded program's metrics happen to match -- rare, but the
// tag must be stripped regardless so the alias lookup below sees the
// real font name.
func stripSubsetTag(name string) string {
	if len(name) > 7 && name[6] == '+' {
		for i := 0; i < 6; i++ {
			if c := name[i]; c < 'A' || c > 'Z' {
				return name
			}
		}
		return name[7:]
	}
	return name
}

// normalizeStandard14Key turns a raw BaseFont string into the lookup key
// used by standard14Aliases.
func normalizeStandard14Key(baseFont string) string {
	s := stripSubsetTag(baseFont)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// standard14WidthsByUnicode holds, per canonical standard-font name, the
// afmGlyphWidths table resolved from AGL glyph name to Unicode rune.
// Built once in init() by running every glyph name through
// AdobeGlyphToUnicode -- the same resolver the encoding tables use, so
// there's a single source of glyph-name-to-Unicode truth in this
// package rather than a second copy of AGL logic living here.
var standard14WidthsByUnicode map[string]map[rune]float64

func init() {
	standard14WidthsByUnicode = make(map[string]map[rune]float64, len(afmGlyphWidths))
	for font, byName := range afmGlyphWidths {
		byRune := make(map[rune]float64, len(byName))
		for gname, w := range byName {
			u := AdobeGlyphToUnicode(gname)
			if u == "" {
				continue
			}
			r, size := utf8.DecodeRuneInString(u)
			if size != len(u) || r == utf8.RuneError {
				// Multi-rune expansions or unresolved names: none
				// expected among the standard 14's glyph names, but
				// skip rather than mis-key if one ever appears. That
				// single glyph then falls through to Font.CharWidth's
				// existing DefaultWidth / flat-500 fallback.
				continue
			}
			byRune[r] = w
		}
		standard14WidthsByUnicode[font] = byRune
	}
}

// Standard14Widths returns the Unicode-rune-keyed AFM width table for
// baseFont if it names -- directly, via a subset tag, or via a common
// substitute-font alias like "Arial" or "TimesNewRoman,Bold" -- one of
// the 14 standard PDF fonts. ok is false for anything not recognized,
// including narrow/condensed variants (see standard14Aliases).
func Standard14Widths(baseFont string) (widths map[rune]float64, ok bool) {
	canonical, found := standard14Aliases[normalizeStandard14Key(baseFont)]
	if !found {
		return nil, false
	}
	w, found := standard14WidthsByUnicode[canonical]
	return w, found
}
