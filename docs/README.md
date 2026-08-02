# Documentation

| | |
| --- | --- |
| [`evaluations/`](evaluations/) | dated accuracy findings — what was measured, against what, and what it does not prove |

User-facing documentation lives in the root [`README.md`](../README.md);
release history in [`CHANGELOG.md`](../CHANGELOG.md).

## Evaluations

| date | subject | headline |
| --- | --- | --- |
| [2026-08-02](evaluations/2026-08-02-icdar2013-table-structure.md) | ICDAR 2013 table detection + structure | F1 0.362 end-to-end; the bottleneck is **detection**, not cell accuracy |
| [2026-08-02](evaluations/2026-08-02-strategy-auto-negative-result.md) | `StrategyAuto` for one-axis-ruled tables | **negative result** — detection 22%→18% missed, but F1 0.362→0.358. Shipped opt-in only |
| [2026-08-02](evaluations/2026-08-02-font-metrics-and-table-fidelity.md) | font metrics and table fidelity | position drift 11.99pt → **0.0000pt**; negative-sign loss 19% → **0%** |

### Conventions

Each report states:

- the **date** and the **commit** it ran against,
- the **dataset** and the **external reference** used as an oracle,
- the **metric**, precisely, and **what the number is not comparable to**,
- **what remains untested**.

That last section is the point. A benchmark result without its scope reads
as a general claim, and every number here is narrower than it looks —
these results say nothing about scanned documents or CJK text, for
instance.

Reports are append-only. Re-running a benchmark adds a new dated file
rather than editing an old one, so a regression is visible as a diff
between two reports instead of vanishing into a rewrite.

### Why findings live here and not only in the issue tracker

An issue records that something was *decided*. A report records what was
*measured*, so the next person can tell whether a number still holds
without re-deriving it — and can see the caveats that made it honest.
