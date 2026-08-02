// Emit every table pdftable finds in a PDF, as JSON, for benchmarking.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/hallelx2/pdftable"
)

type tableOut struct {
	Page int        `json:"page"`
	Rows [][]string `json:"rows"`
}

// edgeSet is one page's explicit row/column boundaries, in PDF points.
type edgeSet struct {
	V []float64 `json:"v"`
	H []float64 `json:"h"`
}

func main() {
	strategy := flag.String("strategy", "lines",
		"lines | text | mixed | auto | lines-then-mixed | fallback")
	merge := flag.Bool("merge", false, "TableSettings.MergeSplitTokens")
	oracle := flag.String("oracle", "",
		`JSON of per-page explicit edges: {"1":{"v":[..],"h":[..]}}`)
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
	// alignment gives the columns.
	mixed := mk(pdftable.StrategyText, pdftable.StrategyLines)
	auto := mk(pdftable.StrategyAuto, pdftable.StrategyAuto)

	var attempts []pdftable.TableSettings
	switch *strategy {
	case "text":
		attempts = []pdftable.TableSettings{text}
	case "mixed":
		attempts = []pdftable.TableSettings{mixed}
	case "auto":
		attempts = []pdftable.TableSettings{auto}
	case "lines-then-mixed":
		attempts = []pdftable.TableSettings{lines, mixed}
	case "fallback":
		attempts = []pdftable.TableSettings{lines, text}
	default:
		attempts = []pdftable.TableSettings{lines}
	}

	// Oracle mode: the caller supplies the row/column boundaries and
	// pdftable only fills the cells. This is exactly the shape of the
	// hybrid a layout model would drive — and fed GROUND-TRUTH edges it
	// measures the ceiling that hybrid can reach: how good extraction gets
	// if detection and gridding were solved perfectly.
	var oracleEdges map[string]edgeSet
	if *oracle != "" {
		if b, err := os.ReadFile(*oracle); err == nil {
			_ = json.Unmarshal(b, &oracleEdges)
		}
	}

	out := []tableOut{}
	for i := 1; i <= doc.NumPages(); i++ {
		p, err := doc.Page(i)
		if err != nil {
			continue
		}

		if oracleEdges != nil {
			e, ok := oracleEdges[strconv.Itoa(i)]
			if !ok || len(e.V) < 2 || len(e.H) < 2 {
				continue
			}
			s := mk(pdftable.StrategyExplicit, pdftable.StrategyExplicit)
			s.ExplicitVerticalLines = e.V
			s.ExplicitHorizontalLines = e.H
			if tables, err := p.ExtractTables(s); err == nil {
				for _, t := range tables {
					out = append(out, tableOut{Page: i, Rows: t.Rows})
				}
			}
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
