package explorer

import (
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type Handler struct {
	Service *Service
}

func (h *Handler) handleRangeQuery(
	w http.ResponseWriter,
	r *http.Request,
	errMessage string,
	query func(tenantID, startMs, endMs int64) (any, error),
) {
	tenantID := modulecommon.Tenant(r).TenantID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := query(tenantID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, errMessage, err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) GetDatastoreSystems(w http.ResponseWriter, r *http.Request) {
	h.handleRangeQuery(w, r, "Failed to query datastore systems", func(tenantID, startMs, endMs int64) (any, error) {
		return h.Service.GetDatastoreSystems(r.Context(), tenantID, startMs, endMs)
	})
}
