package httputil

import (
	"context"
	"fmt"
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	"golang.org/x/sync/errgroup"
)

func HandleRangeQuery(
	w http.ResponseWriter,
	r *http.Request,
	errMessage string,
	query func(ctx context.Context, tenantID, startMs, endMs int64) (any, error),
) {
	tenantID := Tenant(r).TenantID
	startMs, endMs, ok := ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := query(r.Context(), tenantID, startMs, endMs)
	if err != nil {
		RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, errMessage, err)
		return
	}
	RespondOK(w, resp)
}

func HandleComparableRangeQuery(
	w http.ResponseWriter,
	r *http.Request,
	errMessage string,
	query func(ctx context.Context, tenantID, startMs, endMs int64) (any, error),
) {
	tenantID := Tenant(r).TenantID
	startMs, endMs, ok := ParseRequiredRange(w, r)
	if !ok {
		return
	}

	cmpStart, cmpEnd, hasCmp := ParseComparisonRange(r, startMs, endMs)
	if !hasCmp {
		resp, err := query(r.Context(), tenantID, startMs, endMs)
		if err != nil {
			RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, errMessage, err)
			return
		}
		RespondOK(w, resp)
		return
	}

	var primary, comparison any
	group, groupCtx := errgroup.WithContext(r.Context())
	group.Go(func() error {
		var err error
		primary, err = query(groupCtx, tenantID, startMs, endMs)
		return err
	})
	group.Go(func() error {
		var err error
		comparison, err = query(groupCtx, tenantID, cmpStart, cmpEnd)
		if err != nil {
			return fmt.Errorf("comparison query: %w", err)
		}
		return nil
	})
	if err := group.Wait(); err != nil {
		RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, errMessage, err)
		return
	}
	RespondOKWithComparison(w, primary, comparison)
}
