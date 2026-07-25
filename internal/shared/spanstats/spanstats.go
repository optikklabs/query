// Package spanstats holds canonical predicates for the span_stats rollup
// tables (spans -> span_stats_1m/5m/1h). The rollup carries every APM
// dimension as a real column, replacing the former fingerprint join against
// metrics_series for span-derived (RED) reads.
package spanstats

// ErrorPred marks a span-stats row as an error request. status_code_string
// carries the OTel span status promoted from spans ('OK' | 'ERROR' | 'UNSET').
const ErrorPred = "(status_code_string = 'ERROR')"

// DBSpanPred selects database client spans.
const DBSpanPred = "(db_system != '')"
