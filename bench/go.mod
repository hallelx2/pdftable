// Separate module on purpose: the benchmark harness must never add a
// dependency to the library it measures. Nested modules are excluded from
// the parent's ./... , so go build ./... at the repo root ignores this
// entirely and a consumer of pdftable never fetches it.
module github.com/hallelx2/pdftable/bench

go 1.25.0

require github.com/hallelx2/pdftable v0.0.0

replace github.com/hallelx2/pdftable => ../
