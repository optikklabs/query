package explorer

import (
	"errors"
	"fmt"

	"github.com/optikklabs/query/internal/modules/metrics/filter"
)

const (
	maxQueriesPerRequest = 10
	maxGroupByKeys       = 3
	maxFiltersPerQuery   = 20
)

var validSteps = map[string]bool{
	"": true, "1m": true, "5m": true, "15m": true, "1h": true, "1d": true,
}

func validateQueryRequest(req FEQueryRequest) error {
	if req.StartTime <= 0 || req.EndTime <= 0 {
		return errors.New("startTime and endTime are required")
	}
	if req.EndTime <= req.StartTime {
		return errors.New("endTime must be greater than startTime")
	}
	if req.EndTime-req.StartTime > filter.MaxTimeRangeMs {
		return errors.New("time range must not exceed 30 days")
	}
	if !validSteps[req.Step] {
		return fmt.Errorf("unsupported step %q", req.Step)
	}
	if len(req.Queries) == 0 {
		return errors.New("at least one query is required")
	}
	if len(req.Queries) > maxQueriesPerRequest {
		return fmt.Errorf("at most %d queries are allowed", maxQueriesPerRequest)
	}

	ids := make(map[string]struct{}, len(req.Queries))
	for i, query := range req.Queries {
		if query.ID == "" {
			return fmt.Errorf("query %d: id is required", i+1)
		}
		if _, exists := ids[query.ID]; exists {
			return fmt.Errorf("query %q: id must be unique", query.ID)
		}
		ids[query.ID] = struct{}{}
		if len(query.GroupBy) > maxGroupByKeys {
			return fmt.Errorf("query %q: at most %d group-by keys are allowed", query.ID, maxGroupByKeys)
		}
		if len(query.Where) > maxFiltersPerQuery {
			return fmt.Errorf("query %q: at most %d filters are allowed", query.ID, maxFiltersPerQuery)
		}
	}
	return nil
}
