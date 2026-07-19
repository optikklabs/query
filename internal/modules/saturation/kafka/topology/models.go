package topology

// ProducerNode is a service that publishes to one or more topics.
type ProducerNode struct {
	Service    string  `json:"service"`
	RatePerSec float64 `json:"ratePerSec"`
	ErrorRate  float64 `json:"errorRate"`
	P50Ms      float64 `json:"p50Ms"`
	P95Ms      float64 `json:"p95Ms"`
	P99Ms      float64 `json:"p99Ms"`
}

type TopicNode struct {
	Topic              string  `json:"topic"`
	RatePerSec         float64 `json:"ratePerSec"`
	ProducerCount      int     `json:"producerCount"`
	ConsumerGroupCount int     `json:"consumerGroupCount"`
}

type ConsumerNode struct {
	Service    string  `json:"service"`
	Group      string  `json:"group"`
	RatePerSec float64 `json:"ratePerSec"`
	ErrorRate  float64 `json:"errorRate"`
	P50Ms      float64 `json:"p50Ms"`
	P95Ms      float64 `json:"p95Ms"`
	P99Ms      float64 `json:"p99Ms"`
}

type StreamEdge struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Kind       string  `json:"kind"`
	RatePerSec float64 `json:"ratePerSec"`
}

type Pathway struct {
	Producer          string  `json:"producer"`
	Topic             string  `json:"topic"`
	Group             string  `json:"group"`
	Consumer          string  `json:"consumer"`
	ProduceRatePerSec float64 `json:"produceRatePerSec"`
	ConsumeRatePerSec float64 `json:"consumeRatePerSec"`
	ErrorRate         float64 `json:"errorRate"`
}

type TopologyResponse struct {
	Producers []ProducerNode `json:"producers"`
	Topics    []TopicNode    `json:"topics"`
	Consumers []ConsumerNode `json:"consumers"`
	Edges     []StreamEdge   `json:"edges"`
	Pathways  []Pathway      `json:"pathways"`
}
