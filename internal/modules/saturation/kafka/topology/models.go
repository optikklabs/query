package topology

// ProducerNode is a service that publishes to one or more topics.
type ProducerNode struct {
	Service    string  `json:"service"`
	RatePerSec float64 `json:"rate_per_sec"`
	ErrorRate  float64 `json:"error_rate"`
	P95Ms      float64 `json:"p95_ms"`
}

type TopicNode struct {
	Topic              string  `json:"topic"`
	RatePerSec         float64 `json:"rate_per_sec"`
	ProducerCount      int     `json:"producer_count"`
	ConsumerGroupCount int     `json:"consumer_group_count"`
}

type ConsumerNode struct {
	Service    string  `json:"service"`
	Group      string  `json:"group"`
	RatePerSec float64 `json:"rate_per_sec"`
	ErrorRate  float64 `json:"error_rate"`
	P95Ms      float64 `json:"p95_ms"`
}

type StreamEdge struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Kind       string  `json:"kind"`
	RatePerSec float64 `json:"rate_per_sec"`
}

type Pathway struct {
	Producer          string  `json:"producer"`
	Topic             string  `json:"topic"`
	Group             string  `json:"group"`
	Consumer          string  `json:"consumer"`
	ProduceRatePerSec float64 `json:"produce_rate_per_sec"`
	ConsumeRatePerSec float64 `json:"consume_rate_per_sec"`
	ErrorRate         float64 `json:"error_rate"`
}

type TopologyResponse struct {
	Producers []ProducerNode `json:"producers"`
	Topics    []TopicNode    `json:"topics"`
	Consumers []ConsumerNode `json:"consumers"`
	Edges     []StreamEdge   `json:"edges"`
	Pathways  []Pathway      `json:"pathways"`
}
