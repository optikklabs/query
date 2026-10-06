package dispatch

// Payload is the channel-agnostic content of one monitor notification.
type Payload struct {
	MonitorName  string
	Priority     string
	Transition   string // "prev->new" status change, e.g. "ok->alert"
	Status       string
	Value        float64
	Threshold    float64
	ScopeSummary string
	Message      string
	IsAlert      bool
	IsWarning    bool
	IsRecovery   bool
}
