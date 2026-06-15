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
	query func(teamID, startMs, endMs int64) (any, error),
) {
	teamID := modulecommon.Tenant(r).TeamID
	startMs, endMs, ok := modulecommon.ParseRequiredRange(w, r)
	if !ok {
		return
	}
	resp, err := query(teamID, startMs, endMs)
	if err != nil {
		modulecommon.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, errMessage, err)
		return
	}
	modulecommon.RespondOK(w, resp)
}

func (h *Handler) GetDatastoreSystems(w http.ResponseWriter, r *http.Request) {
	h.handleRangeQuery(w, r, "Failed to query datastore systems", func(teamID, startMs, endMs int64) (any, error) {
		return h.Service.GetDatastoreSystems(r.Context(), teamID, startMs, endMs)
	})
}
