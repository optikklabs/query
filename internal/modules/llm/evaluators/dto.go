package evaluators

import "time"

// Evaluator is a scoring definition plus rolling analytics over its emitted
// scores. judge_model is stored for the upcoming automated runner; scores are
// currently ingest-side only.
type Evaluator struct {
	ID             int64            `json:"id"`
	Name           string           `json:"name"`
	ScoreName      string           `json:"scoreName"`
	JudgeModel     string           `json:"judgeModel,omitempty"`
	Target         string           `json:"target"`
	SamplingPct    int              `json:"samplingPct"`
	DataType       string           `json:"dataType"`
	Categories     []string         `json:"categories"`
	PromptTemplate string           `json:"promptTemplate,omitempty"`
	Enabled        bool             `json:"enabled"`
	Analytics      EvaluatorMetrics `json:"analytics"`
	CreatedAt      time.Time        `json:"createdAt"`
	UpdatedAt      time.Time        `json:"updatedAt"`
}

// EvaluatorMetrics are the rolling score aggregates for the evaluator window.
type EvaluatorMetrics struct {
	Count     int64   `json:"count"`
	MeanValue float64 `json:"meanValue"`
}

// UpsertRequest authors or replaces an evaluator definition. For PATCH, unset
// pointer fields are left unchanged.
type UpsertRequest struct {
	Name           string   `json:"name"`
	ScoreName      string   `json:"scoreName"`
	JudgeModel     string   `json:"judgeModel,omitempty"`
	Target         string   `json:"target,omitempty"`
	SamplingPct    *int     `json:"samplingPct,omitempty"`
	DataType       string   `json:"dataType,omitempty"`
	Categories     []string `json:"categories,omitempty"`
	PromptTemplate string   `json:"promptTemplate,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
}

type evaluatorRow struct {
	ID             int64      `db:"id"`
	Name           string     `db:"name"`
	ScoreName      string     `db:"score_name"`
	JudgeModel     *string    `db:"judge_model"`
	Target         string     `db:"target"`
	SamplingPct    int        `db:"sampling_pct"`
	DataType       string     `db:"data_type"`
	CategoriesJSON []byte     `db:"categories_json"`
	PromptTemplate *string    `db:"prompt_template"`
	Enabled        bool       `db:"enabled"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      *time.Time `db:"updated_at"`
}
