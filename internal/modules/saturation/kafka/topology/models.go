package topology

// ProducerNode is a service that publishes to one or more topics.
type ProducerNode struct {
	Service    string  `json:"service"`
	RatePerSec float64 `json:"rate_per_sec"`
	ErrorRate  float64 `json:"error_rate"` // fraction in [0, 1]
	P95Ms      float64 `json:"p95_ms"`
}

// TopicNode is a Kafka topic with its produce throughput and fan-out counts.
type TopicNode struct {
	Topic              string  `json:"topic"`
	RatePerSec         float64 `json:"rate_per_sec"` // produce throughput
	ProducerCount      int     `json:"producer_count"`
	ConsumerGroupCount int     `json:"consumer_group_count"`
}

// ConsumerNode is a service consuming a topic under a consumer group.
type ConsumerNode struct {
	Service    string  `json:"service"`
	Group      string  `json:"group"`
	RatePerSec float64 `json:"rate_per_sec"`
	ErrorRate  float64 `json:"error_rate"`
	P95Ms      float64 `json:"p95_ms"`
}

// StreamEdge is a directed producer->topic or topic->consumer relationship.
type StreamEdge struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Kind       string  `json:"kind"` // "produce" | "consume"
	RatePerSec float64 `json:"rate_per_sec"`
}

// Pathway is one producer->topic->group->consumer flow for the pathways table.
type Pathway struct {
	Producer          string  `json:"producer"`
	Topic             string  `json:"topic"`
	Group             string  `json:"group"`
	Consumer          string  `json:"consumer"`
	ProduceRatePerSec float64 `json:"produce_rate_per_sec"`
	ConsumeRatePerSec float64 `json:"consume_rate_per_sec"`
	ErrorRate         float64 `json:"error_rate"`
}

// TopologyResponse is the payload for GET /saturation/kafka/topology.
type TopologyResponse struct {
	Producers []ProducerNode `json:"producers"`
	Topics    []TopicNode    `json:"topics"`
	Consumers []ConsumerNode `json:"consumers"`
	Edges     []StreamEdge   `json:"edges"`
	Pathways  []Pathway      `json:"pathways"`
}
