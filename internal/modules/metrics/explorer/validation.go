package explorer

import (
	"fmt"

	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

func invalid(format string, args ...any) error {
	return errorcode.ValidationError{Msg: fmt.Sprintf(format, args...)}
}

const (
	maxQueriesPerRequest = 10
	maxGroupByKeys       = 3
	maxFiltersPerQuery   = 20
)

var validSteps = map[string]bool{
	"": true, "1m": true, "5m": true, "15m": true, "1h": true, "1d": true,
}

func validateQueryRequest(req QueryRequest) error {
	if err := filterutil.ValidateTimeRange(req.StartTime, req.EndTime); err != nil {
		return err
	}
	if !validSteps[req.Step] {
		return invalid("unsupported step %q", req.Step)
	}
	if len(req.Queries) == 0 {
		return invalid("at least one query is required")
	}
	if len(req.Queries) > maxQueriesPerRequest {
		return invalid("at most %d queries are allowed", maxQueriesPerRequest)
	}

	ids := make(map[string]struct{}, len(req.Queries))
	for i, query := range req.Queries {
		if query.ID == "" {
			return invalid("query %d: id is required", i+1)
		}
		if _, exists := ids[query.ID]; exists {
			return invalid("query %q: id must be unique", query.ID)
		}
		ids[query.ID] = struct{}{}
		if len(query.GroupBy) > maxGroupByKeys {
			return invalid("query %q: at most %d group-by keys are allowed", query.ID, maxGroupByKeys)
		}
		if len(query.Where) > maxFiltersPerQuery {
			return invalid("query %q: at most %d filters are allowed", query.ID, maxFiltersPerQuery)
		}
	}
	return nil
}
