package tenant

type TenantResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Active    bool   `json:"active"`
	APIKey    string `json:"api_key"`
	CreatedAt any    `json:"created_at"`
}
