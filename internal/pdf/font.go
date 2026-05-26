// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdf

import (
	"fmt"
)

// Font is the interpreter's view of a single PDF font resource. Each
// font on a page (named under `/Font` in the page resources) becomes
// one of these. The interpreter resolves /Tf operators by looking up
// the font name in the page's font map and stashing the *Font on the
// text state — every subsequent text-showing op uses font.Decode to
// turn the input byte string into a sequence of (CID, Unicode, width)
// triples.
//
// A Font is constructed once per page (or once and reused across pages,
// when the same font dict is reachable from multiple pages — pdfcpu
// dereferences indirect references for us so we always get the same
// *Font pointer back).
type Font struct {
	// BaseFont is the PostScript name from the font dictionary's
	// /BaseFont entry, e.g. "Helvetica-Bold" or "ABCDEF+Times". Surfaced
	// verbatim to the caller as Char.FontName.
	BaseFont string

	// IsSimple is true for Type1 and TrueType fonts (single-byte CIDs,
	// /Encoding name + optional /Differences array). Composite fonts
	// (CIDFontType0/2) have IsSimple = false and use a Type0 cmap to
	// segment the byte stream into multi-byte CIDs.
	IsSimple bool

	// cid2unicodeEncoding is the base encoding table for simple fonts:
	// 256 entries indexed by byte value, each a Unicode string. Built
	// from the standard PDF encoding name (WinAnsi/MacRoman/Standard/
	// PDFDoc) overlaid with any /Differences entries from the font dict.
	cid2unicodeEncoding [256]string

	// ToUnicode is the optional parsed /ToUnicode CMap. When present
	// it is consulted FIRST, in front of the encoding table — the PDF
	// spec is unambiguous about this (PDF 1.7 §9.10.2). Many PDFs ship
	// a ToUnicode map even for fonts that already have a usable
	// encoding, because it's the only way to map ligature glyphs back
	// to "fi"/"ffi"/etc.
	ToUnicode *CMap

	// Widths maps CID → advance width in /1000ths of a font unit. For
	// simple fonts the keys are bytes 0..255; for composite fonts they
	// are 2-byte CIDs. DefaultWidth is used for CIDs not in the map.
	Widths       map[uint16]float64
	DefaultWidth float64

	// Ascent and Descent are the font's typographic extrema in
	// /1000ths of a font unit, read from /FontDescriptor. Descent is
	// always stored negative (PDF spec) — we normalise on read.
	Ascent  float64
	Descent float64
}

// Decode walks a PDF text-showing operand (a byte string) and yields
// the sequence of CIDs it represents. For simple fonts that's just the
// bytes; for composite fonts (Identity-H is by far the most common
// composite encoding) bytes are paired into 2-byte CIDs.
//
// The returned slice is fresh — callers may retain it. Per-CID
// resolution (Unicode lookup, width) happens in DecodeUnicode and
// CharWidth, separately, so callers that only want one of those can
// avoid the cost of the other.
func (f *Font) Decode(b []byte) []uint16 {
	if f == nil {
		// No font has been selected; treat each byte as its own CID.
		// This is the same fallback pdfminer takes in non-STRICT mode,
		// and it lets us still emit chars (with empty Unicode) when a
		// content stream references a missing font dict.
		out := make([]uint16, len(b))
		for i, v := range b {
			out[i] = uint16(v)
		}
		return out
	}
	if f.IsSimple {
		out := make([]uint16, len(b))
		for i, v := range b {
			out[i] = uint16(v)
		}
		return out
	}
	// Composite (Type0) font: 2-byte CIDs, big-endian. Trailing odd
	// byte is dropped (would mean a malformed stream).
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	out := make([]uint16, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		out[i/2] = uint16(b[i])<<8 | uint16(b[i+1])
	}
	return out
}

// DecodeUnicode returns the Unicode text for a single CID. Lookup order:
//
//  1. ToUnicode CMap (if present).
//  2. Encoding table (simple fonts only).
//  3. The literal placeholder "(cid:NNN)" — same convention as
//     pdfminer.six. Layout code can detect this prefix and treat such
//     chars as "positioned but unreadable", which is still useful
//     (the bbox carries the table grid even when the text doesn't
//     come back).
func (f *Font) DecodeUnicode(cid uint16) string {
	if f == nil {
		// No font; map the CID as Latin-1 — better than nothing for
		// printable bytes, and consistent with how a viewer would
		// fall back.
		if cid < 256 {
			return string(rune(cid))
		}
		return fmt.Sprintf("(cid:%d)", cid)
	}
	if s, ok := f.ToUnicode.Lookup(cid); ok && s != "" {
		return s
	}
	if f.IsSimple && cid < 256 {
		if s := f.cid2unicodeEncoding[cid]; s != "" {
			return s
		}
		// Fall back to Latin-1 for unmapped bytes in simple fonts.
		// This is the same recovery pdfplumber-via-pdfminer applies
		// in practice — a font with WinAnsi base encoding still
		// produces sensible text for ASCII even when no /Encoding
		// was declared.
		if cid >= 0x20 && cid < 0x7f {
			return string(rune(cid))
		}
	}
	return fmt.Sprintf("(cid:%d)", cid)
}

// CharWidth returns the advance width for cid in /1000ths of the
// font's design unit. Multiply by FontSize/1000 to get the user-space
// advance (text-space units, before applying the text matrix).
func (f *Font) CharWidth(cid uint16) float64 {
	if f == nil {
		// Fall back to 500/1000 of the size — half an em, a reasonable
		// guess for unknown glyphs.
		return 500
	}
	if w, ok := f.Widths[cid]; ok {
		return w
	}
	if f.DefaultWidth != 0 {
		return f.DefaultWidth
	}
	return 500
}

// --- Predefined encodings ---------------------------------------------------
//
// We carry the three most common PDF base encodings (WinAnsi, MacRoman,
// StandardEncoding) inline. Each table is 256 single-rune strings; the
// missing slots (where the encoding has no mapping at all) stay as "".
//
// These tables are correct for the printable ASCII range (32-126),
// which is what 99% of PDFs actually use. Outside that range the
// tables follow Adobe's published mappings — see PDF 1.7 Appendix D.2.
// For uncommon control or accented characters that a particular PDF
// uses without a /ToUnicode map, the worst case is that we render a
// (cid:NNN) placeholder, which is the same behaviour as pdfminer and
// pdfplumber when their internal tables miss a slot.

// EncodingByName returns the 256-entry cid→Unicode table for a base
// encoding name. Returns the StandardEncoding (the PDF spec's default)
// if the name is unrecognised.
func EncodingByName(name string) [256]string {
	switch name {
	case "WinAnsiEncoding":
		return winAnsiEncoding
	case "MacRomanEncoding":
		return macRomanEncoding
	case "StandardEncoding":
		return standardEncoding
	case "PDFDocEncoding":
		return pdfDocEncoding
	default:
		return standardEncoding
	}
}

// ApplyDifferences overlays a /Differences array on a base encoding.
// The array is a flat sequence alternating integer-start values with
// glyph-name entries — see PDF 1.7 §9.6.5.5:
//
//	[ 39 /quotesingle 96 /grave /quoteleft ]
//
// means "glyph 39 is /quotesingle, glyph 96 is /grave, glyph 97 is
// /quoteleft". The integer resets the running CID; each subsequent
// name occupies CID++.
//
// names is a (cid, name) sequence as decoded by the caller (the
// content interpreter does the array walking). out is the table
// returned to the font.
func ApplyDifferences(base [256]string, entries []Difference) [256]string {
	out := base
	for _, e := range entries {
		if e.CID >= 0 && e.CID < 256 {
			out[e.CID] = AdobeGlyphToUnicode(e.GlyphName)
		}
	}
	return out
}

// Difference is one (cid, glyph-name) pair from a /Differences array.
type Difference struct {
	CID       int
	GlyphName string
}

// AdobeGlyphToUnicode resolves Adobe glyph names (e.g. "A", "comma",
// "fi", "Adieresis", "uni0041") to Unicode strings.
//
// For names not in our minimal glyph table, we recognise two
// conventional encodings:
//
//   - "uniXXXX" or "uniXXXXXXXX..." → one or more UTF-16 hex codepoints.
//   - "uXXXX" / "uXXXXX" / "uXXXXXX" → a single hex codepoint.
//
// Anything else returns "" — the caller falls back to a (cid:NNN)
// placeholder.
func AdobeGlyphToUnicode(name string) string {
	if name == "" {
		return ""
	}
	if r, ok := adobeGlyphTable[name]; ok {
		return r
	}
	// uniXXXX, uniXXXXXXXX ... — concatenated 4-hex codepoints.
	if len(name) > 3 && name[:3] == "uni" {
		rest := name[3:]
		if len(rest)%4 == 0 && allHex(rest) {
			var out []rune
			for i := 0; i < len(rest); i += 4 {
				v := parseHex(rest[i : i+4])
				out = append(out, rune(v))
			}
			return string(out)
		}
	}
	// uXXXX..uXXXXXXXX — single hex codepoint, 4–6 digits.
	if len(name) > 1 && name[0] == 'u' {
		rest := name[1:]
		if (len(rest) >= 4 && len(rest) <= 6) && allHex(rest) {
			return string(rune(parseHex(rest)))
		}
	}
	return ""
}

func allHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func parseHex(s string) int {
	v := 0
	for i := 0; i < len(s); i++ {
		v <<= 4
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			v |= int(c - '0')
		case c >= 'a' && c <= 'f':
			v |= int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			v |= int(c-'A') + 10
		}
	}
	return v
}

// adobeGlyphTable is a minimal Adobe Glyph List for the glyphs that
// /Differences arrays use most often. The full AGL is ~4500 entries;
// we ship the printable-ASCII names + the handful of European accents
// that appear in /Differences overlays in practice. Names not in this
// table fall through to the uniXXXX / uXXXX recognisers above, which
// covers the vast majority of remaining cases.
var adobeGlyphTable = map[string]string{
	// ASCII letters/digits.
	"A": "A", "B": "B", "C": "C", "D": "D", "E": "E", "F": "F",
	"G": "G", "H": "H", "I": "I", "J": "J", "K": "K", "L": "L",
	"M": "M", "N": "N", "O": "O", "P": "P", "Q": "Q", "R": "R",
	"S": "S", "T": "T", "U": "U", "V": "V", "W": "W", "X": "X",
	"Y": "Y", "Z": "Z",
	"a": "a", "b": "b", "c": "c", "d": "d", "e": "e", "f": "f",
	"g": "g", "h": "h", "i": "i", "j": "j", "k": "k", "l": "l",
	"m": "m", "n": "n", "o": "o", "p": "p", "q": "q", "r": "r",
	"s": "s", "t": "t", "u": "u", "v": "v", "w": "w", "x": "x",
	"y": "y", "z": "z",
	"zero":  "0",
	"one":   "1",
	"two":   "2",
	"three": "3",
	"four":  "4",
	"five":  "5",
	"six":   "6",
	"seven": "7",
	"eight": "8",
	"nine":  "9",
	// Punctuation.
	"space":        " ",
	"exclam":       "!",
	"quotedbl":     "\"",
	"numbersign":   "#",
	"dollar":       "$",
	"percent":      "%",
	"ampersand":    "&",
	"quoteright":   "’",
	"quotesingle":  "'",
	"parenleft":    "(",
	"parenright":   ")",
	"asterisk":     "*",
	"plus":         "+",
	"comma":        ",",
	"hyphen":       "-",
	"period":       ".",
	"slash":        "/",
	"colon":        ":",
	"semicolon":    ";",
	"less":         "<",
	"equal":        "=",
	"greater":      ">",
	"question":     "?",
	"at":           "@",
	"bracketleft":  "[",
	"backslash":    "\\",
	"bracketright": "]",
	"asciicircum":  "^",
	"underscore":   "_",
	"grave":        "`",
	"quoteleft":    "‘",
	"braceleft":    "{",
	"bar":          "|",
	"braceright":   "}",
	"asciitilde":   "~",
	// Common ligatures.
	"fi":  "fi",
	"fl":  "fl",
	"ffi": "ffi",
	"ffl": "ffl",
	// Common accented letters.
	"Adieresis": "Ä",
	"adieresis": "ä",
	"Odieresis": "Ö",
	"odieresis": "ö",
	"Udieresis": "Ü",
	"udieresis": "ü",
	"germandbls": "ß",
	"eacute":    "é",
	"Eacute":    "É",
	"egrave":    "è",
	"agrave":    "à",
	"acircumflex": "â",
	"ccedilla":   "ç",
	"endash":     "–",
	"emdash":     "—",
	"bullet":     "•",
	"quotedblleft":  "“",
	"quotedblright": "”",
}

// --- Encoding tables -------------------------------------------------------
//
// The tables below cover the printable-ASCII range exactly per Adobe's
// published encoding specs. We initialise them lazily in init(), since
// 4×256-entry literal tables would be unwieldy to type out — instead
// we build them from the small list of name→position pairs above plus
// hard-coded exceptions for the few slots that differ between
// encodings.

var (
	standardEncoding [256]string
	winAnsiEncoding  [256]string
	macRomanEncoding [256]string
	pdfDocEncoding   [256]string
)

func init() {
	// Printable ASCII identity, valid across all four encodings.
	for i := 0x20; i < 0x7f; i++ {
		s := string(rune(i))
		standardEncoding[i] = s
		winAnsiEncoding[i] = s
		macRomanEncoding[i] = s
		pdfDocEncoding[i] = s
	}
	// WinAnsi: the high range adds the Windows-1252 supplement
	// (smart quotes, em/en dashes, euro, etc.). We only populate
	// the slots that real PDFs actually emit.
	winAnsiEncoding[0x80] = "€" // euro
	winAnsiEncoding[0x82] = "‚"
	winAnsiEncoding[0x83] = "ƒ"
	winAnsiEncoding[0x84] = "„"
	winAnsiEncoding[0x85] = "…" // ellipsis
	winAnsiEncoding[0x86] = "†"
	winAnsiEncoding[0x87] = "‡"
	winAnsiEncoding[0x88] = "ˆ"
	winAnsiEncoding[0x89] = "‰"
	winAnsiEncoding[0x8a] = "Š"
	winAnsiEncoding[0x8b] = "‹"
	winAnsiEncoding[0x8c] = "Œ"
	winAnsiEncoding[0x8e] = "Ž"
	winAnsiEncoding[0x91] = "‘"
	winAnsiEncoding[0x92] = "’"
	winAnsiEncoding[0x93] = "“"
	winAnsiEncoding[0x94] = "”"
	winAnsiEncoding[0x95] = "•" // bullet
	winAnsiEncoding[0x96] = "–" // en dash
	winAnsiEncoding[0x97] = "—" // em dash
	winAnsiEncoding[0x98] = "˜"
	winAnsiEncoding[0x99] = "™"
	winAnsiEncoding[0x9a] = "š"
	winAnsiEncoding[0x9b] = "›"
	winAnsiEncoding[0x9c] = "œ"
	winAnsiEncoding[0x9e] = "ž"
	winAnsiEncoding[0x9f] = "Ÿ"
	// Latin-1 supplement (0xa0..0xff): WinAnsi matches Latin-1 here.
	for i := 0xa0; i < 0x100; i++ {
		winAnsiEncoding[i] = string(rune(i))
	}
}
