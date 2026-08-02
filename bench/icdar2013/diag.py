"""Separate the two ways table extraction can fail on ICDAR 2013.

Low recall has two very different causes and the headline F1 cannot tell
them apart:

  (a) the table was never detected at all — a detection problem
  (b) it was detected but gridded wrongly — a structure problem

(a) is fixed by better "is this a table?" logic; (b) by better cell
geometry. They need opposite work, so it is worth knowing which one we
have before touching anything.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
from collections import Counter

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from score import gt_relations, norm, relations_from_grid, score  # noqa: E402


def main() -> int:
    root, exe = sys.argv[1], sys.argv[2]
    pairs = []
    for dirpath, _, files in os.walk(root):
        for f in sorted(files):
            if f.endswith("-str.xml"):
                pdf = os.path.join(dirpath, f.replace("-str.xml", ".pdf"))
                if os.path.exists(pdf):
                    pairs.append((pdf, os.path.join(dirpath, f)))
    pairs.sort()

    zero_detect = 0
    gt_regions_total = 0
    detected_tables_total = 0
    pages_total = 0
    per_doc = []

    for pdf, xml in pairs:
        gt = gt_relations(xml)
        out = subprocess.run(
            [exe, "-strategy", os.environ.get("DIAG_STRATEGY","lines"), pdf], capture_output=True, timeout=120
        ).stdout
        tables = json.loads(out or b"[]")
        detected_tables_total += len(tables)
        rels: Counter = Counter()
        for t in tables:
            rels += relations_from_grid([[norm(c) for c in r] for r in t["rows"]])
        c, nd, ng = score(gt, rels)
        if nd == 0:
            zero_detect += 1
        gt_regions_total += 1
        per_doc.append((os.path.basename(pdf), ng, nd, c, len(tables)))

    print(f"documents                     : {len(pairs)}")
    print(f"documents where we found NO table: {zero_detect}"
          f"  ({100*zero_detect/len(pairs):.0f}%)")
    print(f"tables detected (total)       : {detected_tables_total}")
    print()

    # Restrict to documents where we DID detect something: if structure is
    # good, precision AND recall should both be high on this subset.
    tot = [0, 0, 0]
    for _, ng, nd, c, _ in per_doc:
        if nd:
            tot[0] += c
            tot[1] += nd
            tot[2] += ng
    p = tot[0] / tot[1] if tot[1] else 0
    r = tot[0] / tot[2] if tot[2] else 0
    f = 2 * p * r / (p + r) if (p + r) else 0
    print("--- documents where a table WAS detected ---")
    print(f"  precision {p:.3f}   recall {r:.3f}   F1 {f:.3f}")
    print()
    print("worst 8 documents by missed relations:")
    for name, ng, nd, c, nt in sorted(per_doc, key=lambda x: x[1] - x[3])[-8:]:
        print(f"  {name:<18} gt={ng:<6} detected={nd:<6} correct={c:<6} tables={nt}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
