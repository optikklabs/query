package spanstats

const ErrorPred = "(status_code_string = 'ERROR')"

const DBSpanPred = "(db_system != '')"

// RED counts inbound work; CLIENT/PRODUCER spans are the callee's requests.
const InboundPred = "(kind_string IN ('SERVER', 'CONSUMER'))"
