package httputil

import (
	"context"
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
)

// HandleRangeQuery parses tenant and required range, executes query, responds.
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
