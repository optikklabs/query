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

func BindFiltered[T FilteredRequest](w http.ResponseWriter, r *http.Request, req T) bool {
	if err := DecodeJSON(r, req); err != nil {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body", nil)
		return false
	}
	if err := req.BindTenant(Tenant(r).TenantID); err != nil {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid filters", err)
		return false
	}
	return true
}

func ValidateSuggestRequest(w http.ResponseWriter, r *http.Request, req *filterutil.SuggestRequest) bool {
	if req.StartTime <= 0 || req.EndTime <= 0 || req.StartTime >= req.EndTime {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Valid startTime and endTime are required", nil)
		return false
	}
	if req.EndTime-req.StartTime > filterutil.RawRetentionMs {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "time range must not exceed retained 15 days", nil)
		return false
	}
	req.Field = strings.TrimSpace(req.Field)
	if req.Field == "" {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "field is required", nil)
		return false
	}
	return true
}

func BindSuggestRequest(w http.ResponseWriter, r *http.Request, req *filterutil.SuggestRequest, isScalar func(string) bool) bool {
	if err := DecodeJSON(r, req); err != nil {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "Invalid request body", nil)
		return false
	}
	if !ValidateSuggestRequest(w, r, req) {
		return false
	}
	if !strings.HasPrefix(req.Field, "@") && !isScalar(req.Field) {
		RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "unknown field", nil)
		return false
	}
	return true
}
