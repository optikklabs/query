package prompts

import (
	"encoding/json"
	"time"
)

// PromptSummary is a catalog row: the prompt plus its production version number.
type PromptSummary struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	Type              string    `json:"type"`
	Description       string    `json:"description,omitempty"`
	Tags              []string  `json:"tags"`
	VersionCount      int       `json:"versionCount"`
	ProductionVersion *int      `json:"productionVersion,omitempty"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// PromptDetail returns the prompt with its full version history.
type PromptDetail struct {
	PromptSummary
	Versions []PromptVersion `json:"versions"`
}

// PromptVersion is a single immutable revision of a prompt template.
type PromptVersion struct {
	Version   int             `json:"version"`
	Template  json.RawMessage `json:"template"`
	Variables []string        `json:"variables"`
	Notes     string          `json:"notes,omitempty"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"createdAt"`
}

// CreatePromptRequest authors a prompt and its first (v1) version.
type CreatePromptRequest struct {
	Name        string          `json:"name"`
	Type        string          `json:"type,omitempty"`
	Description string          `json:"description,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Template    json.RawMessage `json:"template"`
	Variables   []string        `json:"variables,omitempty"`
	Notes       string          `json:"notes,omitempty"`
}

// CreateVersionRequest appends a new version to an existing prompt.
type CreateVersionRequest struct {
	Template  json.RawMessage `json:"template"`
	Variables []string        `json:"variables,omitempty"`
	Notes     string          `json:"notes,omitempty"`
	// Production, when true, promotes the new version on creation.
	Production bool `json:"production,omitempty"`
}

// UpdateVersionRequest changes a version's lifecycle status.
type UpdateVersionRequest struct {
	Status string `json:"status"`
}

// --- DB row shapes ---

type promptRow struct {
	ID          int64      `db:"id"`
	Name        string     `db:"name"`
	Type        string     `db:"type"`
	Description *string    `db:"description"`
	TagsJSON    []byte     `db:"tags_json"`
	UpdatedAt   *time.Time `db:"updated_at"`
	CreatedAt   time.Time  `db:"created_at"`
}

type versionRow struct {
	Version       int       `db:"version"`
	TemplateJSON  []byte    `db:"template_json"`
	VariablesJSON []byte    `db:"variables_json"`
	Notes         *string   `db:"notes"`
	Status        string    `db:"status"`
	CreatedAt     time.Time `db:"created_at"`
}
