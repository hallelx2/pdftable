"""Generate golden-file expected outputs for pdftable's parity tests.

Run from the repo root after copying the fixture PDFs into
testdata/golden/:

    pip install pdfplumber
    python scripts/gen_golden.py

The script reads every *.pdf in testdata/golden/, runs pdfplumber's
extract_text() and extract_words() on each page, and writes the result
as <name>.expected.json next to the PDF.

Coordinate-system note: pdfplumber emits word "top" and "bottom" in
image space (origin at top-left, Y growing DOWN). pdftable uses PDF
user space (origin at bottom-left, Y growing UP). We translate
pdfplumber's coords into PDF-user-space here so the JSON matches the
y0/y1 fields on pdftable.Word directly.

To regenerate after upgrading pdfplumber, simply re-run this script.
The file outputs are deterministic and stable.
"""

from __future__ import annotations

import json
import os
import sys

import pdfplumber

DIR = os.path.join("testdata", "golden")


def main() -> int:
    target = DIR if len(sys.argv) < 2 else sys.argv[1]
    pdfs = sorted(
        f for f in os.listdir(target) if f.endswith(".pdf")
    )
    if not pdfs:
        print(f"no .pdf files in {target}", file=sys.stderr)
        return 1
    for fname in pdfs:
        name = os.path.splitext(fname)[0]
        pdf_path = os.path.join(target, fname)
        out = {"name": name, "pages": []}
        with pdfplumber.open(pdf_path) as pdf:
            for p in pdf.pages:
                page = {
                    "number": p.page_number,
                    "width": p.width,
                    "height": p.height,
                    "extract_text": p.extract_text() or "",
                    "extract_words": [],
                }
                words = p.extract_words(
                    x_tolerance=3,
                    y_tolerance=3,
                    keep_blank_chars=False,
                    use_text_flow=False,
                    horizontal_ltr=True,
                    vertical_ttb=True,
                    extra_attrs=None,
                    split_at_punctuation=False,
                    expand_ligatures=True,
                )
                for w in words:
                    y1_user = p.height - w["top"]
                    y0_user = p.height - w["bottom"]
                    page["extract_words"].append(
                        {
                            "text": w["text"],
                            "x0": w["x0"],
                            "x1": w["x1"],
                            "y0": y0_user,
                            "y1": y1_user,
                            "upright": bool(w.get("upright", True)),
                            "direction": w.get("direction", "ltr"),
                        }
                    )
                out["pages"].append(page)
        expected = os.path.join(target, f"{name}.expected.json")
        with open(expected, "w", encoding="utf-8") as f:
            json.dump(out, f, ensure_ascii=False, indent=2)
        nwords = sum(len(pp["extract_words"]) for pp in out["pages"])
        print(f"wrote {expected}: {len(out['pages'])} pages, {nwords} words")
    return 0


if __name__ == "__main__":
    sys.exit(main())
