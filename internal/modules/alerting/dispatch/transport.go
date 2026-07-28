package dispatch

type Payload struct {
	MonitorID   int64
	MonitorName string
	MonitorURL  string

	MonitorType string

	Priority string

	Transition string

	Status       string
	Value        float64
	Threshold    float64
	ScopeSummary string

	Message    string
	IsAlert    bool
	IsWarning  bool
	IsRecovery bool
}
