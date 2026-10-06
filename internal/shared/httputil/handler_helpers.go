package httputil

import (
	"net/http"
	"strings"

	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

type FilteredRequest interface {
	BindTenant(tenantID int64) error
}

// BindFiltered decodes a filtered query body and binds it to the caller's
// tenant, answering 400 when either step fails.
func BindFiltered[T FilteredRequest](w http.ResponseWriter, r *http.Request, req T) bool {
	if !BindJSON(w, r, req) {
		return false
	}
	if err := req.BindTenant(Tenant(r).TenantID); err != nil {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, err.Error(), nil)
		return false
	}
	return true
}

// BindSuggestRequest decodes a suggest body and accepts only attribute
// fields ("@key") or the scalar fields isScalar knows.
func BindSuggestRequest(w http.ResponseWriter, r *http.Request, req *filterutil.SuggestRequest, isScalar func(string) bool) bool {
	if !BindJSON(w, r, req) {
		return false
	}
	req.Field = strings.TrimSpace(req.Field)
	rangeErr := filterutil.ValidateTimeRange(req.StartTime, req.EndTime)
	switch {
	case rangeErr != nil:
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, rangeErr.Error(), nil)
	case req.Field == "":
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "field is required", nil)
	case !strings.HasPrefix(req.Field, "@") && !isScalar(req.Field):
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "unknown field", nil)
	default:
		return true
	}
	return false
}
