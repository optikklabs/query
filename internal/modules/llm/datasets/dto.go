package datasets

import (
	"encoding/json"
	"time"
)

// DatasetSummary is a catalog row with item + run counts.
type DatasetSummary struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	ItemCount   int       `json:"itemCount"`
	RunCount    int       `json:"runCount"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// DatasetDetail returns the dataset with its items and recent runs.
type DatasetDetail struct {
	DatasetSummary
	Items []DatasetItem `json:"items"`
	Runs  []RunSummary  `json:"runs"`
}

// DatasetItem is a single input/expected-output test case.
type DatasetItem struct {
	ID             int64           `json:"id"`
	Input          json.RawMessage `json:"input"`
	ExpectedOutput json.RawMessage `json:"expectedOutput,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
}

// RunSummary is a completed/in-flight experiment run over a dataset.
type RunSummary struct {
	ID           int64           `json:"id"`
	Name         string          `json:"name"`
	Provider     string          `json:"provider"`
	Model        string          `json:"model"`
	Status       string          `json:"status"`
	ItemCount    int             `json:"itemCount"`
	AvgScores    json.RawMessage `json:"avgScores"`
	TotalCostUsd float64         `json:"totalCostUsd"`
	AvgLatencyMs float64         `json:"avgLatencyMs"`
	Error        string          `json:"error,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	CompletedAt  *time.Time      `json:"completedAt,omitempty"`
}

// RunDetail returns a run with its per-item results.
type RunDetail struct {
	RunSummary
	Items []RunItem `json:"items"`
}

// RunItem is the model output for one dataset item within a run.
type RunItem struct {
	DatasetItemID int64           `json:"datasetItemId"`
	Output        json.RawMessage `json:"output,omitempty"`
	LatencyMs     int             `json:"latencyMs"`
	CostUsd       float64         `json:"costUsd"`
	Scores        json.RawMessage `json:"scores"`
	Error         string          `json:"error,omitempty"`
}

// CreateDatasetRequest authors a new dataset.
type CreateDatasetRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// AddItemsRequest bulk-appends test cases to a dataset.
type AddItemsRequest struct {
	Items []ItemInput `json:"items"`
}

// ItemInput is a single case in a bulk add.
type ItemInput struct {
	Input          json.RawMessage `json:"input"`
	ExpectedOutput json.RawMessage `json:"expectedOutput,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

// --- DB row shapes ---

type datasetRow struct {
	ID          int64      `db:"id"`
	Name        string     `db:"name"`
	Description *string    `db:"description"`
	ItemCount   int        `db:"item_count"`
	RunCount    int        `db:"run_count"`
	UpdatedAt   *time.Time `db:"updated_at"`
	CreatedAt   time.Time  `db:"created_at"`
}

type itemRow struct {
	ID                 int64     `db:"id"`
	InputJSON          []byte    `db:"input_json"`
	ExpectedOutputJSON []byte    `db:"expected_output_json"`
	MetadataJSON       []byte    `db:"metadata_json"`
	CreatedAt          time.Time `db:"created_at"`
}

type runRow struct {
	ID            int64      `db:"id"`
	Name          string     `db:"name"`
	Provider      string     `db:"provider"`
	Model         string     `db:"model"`
	Status        string     `db:"status"`
	ItemCount     int        `db:"item_count"`
	AvgScoresJSON []byte     `db:"avg_scores_json"`
	TotalCostUsd  float64    `db:"total_cost_usd"`
	AvgLatencyMs  float64    `db:"avg_latency_ms"`
	Error         *string    `db:"error"`
	CreatedAt     time.Time  `db:"created_at"`
	CompletedAt   *time.Time `db:"completed_at"`
}

type runItemRow struct {
	DatasetItemID int64   `db:"dataset_item_id"`
	OutputJSON    []byte  `db:"output_json"`
	LatencyMs     int     `db:"latency_ms"`
	CostUsd       float64 `db:"cost_usd"`
	ScoresJSON    []byte  `db:"scores_json"`
	Error         *string `db:"error"`
}
