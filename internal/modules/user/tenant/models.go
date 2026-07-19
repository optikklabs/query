package tenant

// TenantResponse carries the raw api_key only in the rotate response (show
// once); every other surface gets just the display prefix.
type TenantResponse struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Active       bool   `json:"active"`
	APIKey       string `json:"apiKey,omitempty"`
	APIKeyPrefix string `json:"apiKeyPrefix"`
	CreatedAt    any    `json:"createdAt"`
}
