// Copyright (c) 2026 Halleluyah Oludele
// Licensed under the MIT License.

package pdf

// symbolGlyphTable maps the Symbol font's glyph names to Unicode.
//
// These are genuine Adobe Glyph List entries -- glyphlist.txt carries
// "Alpha;0391", "universal;2200", "club;2663" and the rest -- so they
// belong in the shared name-to-Unicode path rather than in a
// font-scoped table: a /Differences array in ANY font may legitimately
// name "Alpha", and it means U+0391 wherever it appears. That is the
// opposite of the ZapfDingbats "aNN" names, which are font-specific and
// deliberately kept out of the global resolver (see
// zapfDingbatsGlyphTable).
//
// AdobeGlyphToUnicode consults adobeGlyphTable first, so the ~45 names
// shared between the two tables (space, comma, digits, mu, multiply,
// ...) keep their existing values and behaviour is unchanged for them.
//
// Two AGL values look surprising and are correct as transcribed:
// "Delta" is U+2206 INCREMENT (not U+0394) and "Omega" is U+2126 OHM
// SIGN (not U+03A9). That is what Adobe's glyphlist.txt specifies, and
// matching it keeps us byte-compatible with pdfminer.six -- the parity
// target this package is measured against.
//
// Generated from Adobe's glyphlist.txt (agl-aglfn), not hand-typed.
var symbolGlyphTable = map[string]string{
	"Alpha":          "\u0391", // Α
	"Beta":           "\u0392", // Β
	"Chi":            "\u03A7", // Χ
	"Delta":          "\u2206", // ∆
	"Epsilon":        "\u0395", // Ε
	"Eta":            "\u0397", // Η
	"Euro":           "\u20AC", // €
	"Gamma":          "\u0393", // Γ
	"Ifraktur":       "\u2111", // ℑ
	"Iota":           "\u0399", // Ι
	"Kappa":          "\u039A", // Κ
	"Lambda":         "\u039B", // Λ
	"Mu":             "\u039C", // Μ
	"Nu":             "\u039D", // Ν
	"Omega":          "\u2126", // Ω
	"Omicron":        "\u039F", // Ο
	"Phi":            "\u03A6", // Φ
	"Pi":             "\u03A0", // Π
	"Psi":            "\u03A8", // Ψ
	"Rfraktur":       "\u211C", // ℜ
	"Rho":            "\u03A1", // Ρ
	"Sigma":          "\u03A3", // Σ
	"Tau":            "\u03A4", // Τ
	"Theta":          "\u0398", // Θ
	"Upsilon":        "\u03A5", // Υ
	"Upsilon1":       "\u03D2", // ϒ
	"Xi":             "\u039E", // Ξ
	"Zeta":           "\u0396", // Ζ
	"aleph":          "\u2135", // ℵ
	"alpha":          "\u03B1", // α
	"ampersand":      "&",      // &
	"angle":          "\u2220", // ∠
	"angleleft":      "\u2329", // 〈
	"angleright":     "\u232A", // 〉
	"apple":          "\uF8FF", // private use
	"approxequal":    "\u2248", // ≈
	"arrowboth":      "\u2194", // ↔
	"arrowdblboth":   "\u21D4", // ⇔
	"arrowdbldown":   "\u21D3", // ⇓
	"arrowdblleft":   "\u21D0", // ⇐
	"arrowdblright":  "\u21D2", // ⇒
	"arrowdblup":     "\u21D1", // ⇑
	"arrowdown":      "\u2193", // ↓
	"arrowhorizex":   "\uF8E7", // private use
	"arrowleft":      "\u2190", // ←
	"arrowright":     "\u2192", // →
	"arrowup":        "\u2191", // ↑
	"arrowvertex":    "\uF8E6", // private use
	"asteriskmath":   "\u2217", // ∗
	"bar":            "|",      // |
	"beta":           "\u03B2", // β
	"braceex":        "\uF8F4", // private use
	"braceleft":      "{",      // {
	"braceleftbt":    "\uF8F3", // private use
	"braceleftmid":   "\uF8F2", // private use
	"bracelefttp":    "\uF8F1", // private use
	"braceright":     "}",      // }
	"bracerightbt":   "\uF8FE", // private use
	"bracerightmid":  "\uF8FD", // private use
	"bracerighttp":   "\uF8FC", // private use
	"bracketleft":    "[",      // [
	"bracketleftbt":  "\uF8F0", // private use
	"bracketleftex":  "\uF8EF", // private use
	"bracketlefttp":  "\uF8EE", // private use
	"bracketright":   "]",      // ]
	"bracketrightbt": "\uF8FB", // private use
	"bracketrightex": "\uF8FA", // private use
	"bracketrighttp": "\uF8F9", // private use
	"bullet":         "\u2022", // •
	"carriagereturn": "\u21B5", // ↵
	"chi":            "\u03C7", // χ
	"circlemultiply": "\u2297", // ⊗
	"circleplus":     "\u2295", // ⊕
	"club":           "\u2663", // ♣
	"colon":          ":",      // :
	"comma":          ",",      // ,
	"congruent":      "\u2245", // ≅
	"copyrightsans":  "\uF8E9", // private use
	"copyrightserif": "\uF6D9", // private use
	"degree":         "\u00B0", // °
	"delta":          "\u03B4", // δ
	"diamond":        "\u2666", // ♦
	"divide":         "\u00F7", // ÷
	"dotmath":        "\u22C5", // ⋅
	"eight":          "8",      // 8
	"element":        "\u2208", // ∈
	"ellipsis":       "\u2026", // …
	"emptyset":       "\u2205", // ∅
	"epsilon":        "\u03B5", // ε
	"equal":          "=",      // =
	"equivalence":    "\u2261", // ≡
	"eta":            "\u03B7", // η
	"exclam":         "!",      // !
	"existential":    "\u2203", // ∃
	"five":           "5",      // 5
	"florin":         "\u0192", // ƒ
	"four":           "4",      // 4
	"fraction":       "\u2044", // ⁄
	"gamma":          "\u03B3", // γ
	"gradient":       "\u2207", // ∇
	"greater":        ">",      // >
	"greaterequal":   "\u2265", // ≥
	"heart":          "\u2665", // ♥
	"infinity":       "\u221E", // ∞
	"integral":       "\u222B", // ∫
	"integralbt":     "\u2321", // ⌡
	"integralex":     "\uF8F5", // private use
	"integraltp":     "\u2320", // ⌠
	"intersection":   "\u2229", // ∩
	"iota":           "\u03B9", // ι
	"kappa":          "\u03BA", // κ
	"lambda":         "\u03BB", // λ
	"less":           "<",      // <
	"lessequal":      "\u2264", // ≤
	"logicaland":     "\u2227", // ∧
	"logicalnot":     "\u00AC", // ¬
	"logicalor":      "\u2228", // ∨
	"lozenge":        "\u25CA", // ◊
	"minus":          "\u2212", // −
	"minute":         "\u2032", // ′
	"mu":             "\u00B5", // µ
	"multiply":       "\u00D7", // ×
	"nine":           "9",      // 9
	"notelement":     "\u2209", // ∉
	"notequal":       "\u2260", // ≠
	"notsubset":      "\u2284", // ⊄
	"nu":             "\u03BD", // ν
	"numbersign":     "#",      // #
	"omega":          "\u03C9", // ω
	"omega1":         "\u03D6", // ϖ
	"omicron":        "\u03BF", // ο
	"one":            "1",      // 1
	"parenleft":      "(",      // (
	"parenleftbt":    "\uF8ED", // private use
	"parenleftex":    "\uF8EC", // private use
	"parenlefttp":    "\uF8EB", // private use
	"parenright":     ")",      // )
	"parenrightbt":   "\uF8F8", // private use
	"parenrightex":   "\uF8F7", // private use
	"parenrighttp":   "\uF8F6", // private use
	"partialdiff":    "\u2202", // ∂
	"percent":        "%",      // %
	"period":         ".",      // .
	"perpendicular":  "\u22A5", // ⊥
	"phi":            "\u03C6", // φ
	"phi1":           "\u03D5", // ϕ
	"pi":             "\u03C0", // π
	"plus":           "+",      // +
	"plusminus":      "\u00B1", // ±
	"product":        "\u220F", // ∏
	"propersubset":   "\u2282", // ⊂
	"propersuperset": "\u2283", // ⊃
	"proportional":   "\u221D", // ∝
	"psi":            "\u03C8", // ψ
	"question":       "?",      // ?
	"radical":        "\u221A", // √
	"radicalex":      "\uF8E5", // private use
	"reflexsubset":   "\u2286", // ⊆
	"reflexsuperset": "\u2287", // ⊇
	"registersans":   "\uF8E8", // private use
	"registerserif":  "\uF6DA", // private use
	"rho":            "\u03C1", // ρ
	"second":         "\u2033", // ″
	"semicolon":      ";",      // ;
	"seven":          "7",      // 7
	"sigma":          "\u03C3", // σ
	"sigma1":         "\u03C2", // ς
	"similar":        "\u223C", // ∼
	"six":            "6",      // 6
	"slash":          "/",      // /
	"space":          " ",      //
	"spade":          "\u2660", // ♠
	"suchthat":       "\u220B", // ∋
	"summation":      "\u2211", // ∑
	"tau":            "\u03C4", // τ
	"therefore":      "\u2234", // ∴
	"theta":          "\u03B8", // θ
	"theta1":         "\u03D1", // ϑ
	"three":          "3",      // 3
	"trademarksans":  "\uF8EA", // private use
	"trademarkserif": "\uF6DB", // private use
	"two":            "2",      // 2
	"underscore":     "_",      // _
	"union":          "\u222A", // ∪
	"universal":      "\u2200", // ∀
	"upsilon":        "\u03C5", // υ
	"weierstrass":    "\u2118", // ℘
	"xi":             "\u03BE", // ξ
	"zero":           "0",      // 0
	"zeta":           "\u03B6", // ζ
}
