package redfleet

type TopEndpointsCursor struct {
	TotalCount    uint64 `json:"cnt"`
	OperationName string `json:"op"`
}

func (c TopEndpointsCursor) IsZero() bool {
	return c.TotalCount == 0 && c.OperationName == ""
}

type PageInfo struct {
	HasMore    bool   `json:"hasMore"`
	NextCursor string `json:"nextCursor,omitempty"`
	Limit      int    `json:"limit"`
}

type PaginatedEndpoints struct {
	Results  []TopEndpoint `json:"results"`
	PageInfo PageInfo      `json:"pageInfo"`
}
