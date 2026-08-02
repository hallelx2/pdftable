// Emit every table pdftable finds in a PDF, as JSON, for benchmarking.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/hallelx2/pdftable"
)

type tableOut struct {
	Page int        `json:"page"`
	Rows [][]string `json:"rows"`
}

func main() {
	strategy := flag.String("strategy", "lines", "lines | text | fallback")
	merge := flag.Bool("merge", false, "TableSettings.MergeSplitTokens")
	flag.Parse()

	doc, err := pdftable.OpenFile(flag.Arg(0))
	if err != nil {
		// A file we cannot open scores zero rather than aborting the run;
		// the harness needs a result for every document.
		json.NewEncoder(os.Stdout).Encode([]tableOut{})
		fmt.Fprintln(os.Stderr, "open:", err)
		return
	}
	defer doc.Close()

	mk := func(v, h pdftable.TableStrategy) pdftable.TableSettings {
		s := pdftable.DefaultTableSettings()
		s.VerticalStrategy, s.HorizontalStrategy = v, h
		s.MergeSplitTokens = *merge
		return s
	}
	lines := mk(pdftable.StrategyLines, pdftable.StrategyLines)
	text := mk(pdftable.StrategyText, pdftable.StrategyText)

	// "mixed" is the booktabs case: horizontal rules give the rows, word
	// alignment gives the columns. A table ruled only horizontally has no
	// ruling intersections at all, so pure "lines" cannot see it.
	mixed := mk(pdftable.StrategyText, pdftable.StrategyLines)
	auto := mk(pdftable.StrategyAuto, pdftable.StrategyAuto)

	var attempts []pdftable.TableSettings
	switch *strategy {
	case "lines":
		attempts = []pdftable.TableSettings{lines}
	case "text":
		attempts = []pdftable.TableSettings{text}
	case "mixed":
		attempts = []pdftable.TableSettings{mixed}
	case "lines-then-mixed":
		attempts = []pdftable.TableSettings{lines, mixed}
	case "auto":
		attempts = []pdftable.TableSettings{auto}
	default: // fallback: ruled cells first, whitespace alignment if none
		attempts = []pdftable.TableSettings{lines, text}
	}

	out := []tableOut{}
	for i := 1; i <= doc.NumPages(); i++ {
		p, err := doc.Page(i)
		if err != nil {
			continue
		}
		for _, s := range attempts {
			tables, err := p.ExtractTables(s)
			if err != nil || len(tables) == 0 {
				continue
			}
			for _, t := range tables {
				out = append(out, tableOut{Page: i, Rows: t.Rows})
			}
			break
		}
	}
	json.NewEncoder(os.Stdout).Encode(out)
}
