package httputil

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/sync/errgroup"
)

// RangeQuery runs one tenant-scoped query over a time window.
type RangeQuery func(ctx context.Context, tenantID, startMs, endMs int64) (any, error)

// HandleRangeQuery parses the required time range, runs query and writes
// the result, mapping service errors to their HTTP status.
func HandleRangeQuery(w http.ResponseWriter, r *http.Request, errMessage string, query RangeQuery) {
	startMs, endMs, ok := ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := query(r.Context(), Tenant(r).TenantID, startMs, endMs)
	if err != nil {
		RespondServiceError(w, r, err, errMessage)
		return
	}
	RespondOK(w, resp)
}

// HandleComparableRangeQuery is HandleRangeQuery that, when the request asks
// for a comparison window, also runs query over that window in parallel.
func HandleComparableRangeQuery(w http.ResponseWriter, r *http.Request, errMessage string, query RangeQuery) {
	startMs, endMs, ok := ParseRequiredRange(w, r)
	if !ok {
		return
	}
	cmpStart, cmpEnd, hasCmp := ParseComparisonRange(r, startMs, endMs)
	if !hasCmp {
		HandleRangeQuery(w, r, errMessage, query)
		return
	}

	tenantID := Tenant(r).TenantID
	var primary, comparison any
	group, groupCtx := errgroup.WithContext(r.Context())
	group.Go(func() error {
		var err error
		primary, err = query(groupCtx, tenantID, startMs, endMs)
		return err
	})
	group.Go(func() error {
		var err error
		if comparison, err = query(groupCtx, tenantID, cmpStart, cmpEnd); err != nil {
			return fmt.Errorf("comparison query: %w", err)
		}
		return nil
	})
	if err := group.Wait(); err != nil {
		RespondServiceError(w, r, err, errMessage)
		return
	}
	RespondOKWithComparison(w, primary, comparison)
}
