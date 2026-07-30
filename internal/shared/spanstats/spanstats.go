package spanstats

const ErrorPred = "(status_code_string = 'ERROR' OR http_status_bucket = '5xx' OR (kind_string = 'CLIENT' AND http_status_bucket = '4xx'))"

const DBSpanPred = "(db_system != '')"

// RED counts inbound work; CLIENT/PRODUCER spans are the callee's requests.
const InboundPred = "(kind_string IN ('SERVER', 'CONSUMER'))"
