package redfleet

import "github.com/optikklabs/query/internal/shared/contracts"

type TopEndpointsCursor struct {
	TotalCount    uint64 `json:"cnt"`
	OperationName string `json:"op"`
}

func (c TopEndpointsCursor) IsZero() bool {
	return c.TotalCount == 0 && c.OperationName == ""
}

type PaginatedEndpoints struct {
	Results  []TopEndpoint      `json:"results"`
	PageInfo contracts.PageInfo `json:"pageInfo"`
}

type PaginatedDBQueries struct {
	Results  []TopDBQuery       `json:"results"`
	PageInfo contracts.PageInfo `json:"pageInfo"`
}
