package tenant

// TenantResponse carries the raw api_key only in the rotate response (show
// once); every other surface gets just the display prefix.
type TenantResponse struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Active       bool   `json:"active"`
	APIKey       string `json:"api_key,omitempty"`
	APIKeyPrefix string `json:"api_key_prefix"`
	CreatedAt    any    `json:"created_at"`
}
