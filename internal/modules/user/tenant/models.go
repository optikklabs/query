package tenant

type TenantResponse struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Active       bool   `json:"active"`
	APIKey       string `json:"apiKey,omitempty"`
	APIKeyPrefix string `json:"apiKeyPrefix"`
	CreatedAt    any    `json:"createdAt"`
}
