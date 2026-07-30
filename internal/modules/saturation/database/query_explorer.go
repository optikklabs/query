package database

import (
	"errors"
	"net/http"

	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/filterutil"
	modulecommon "github.com/optikklabs/query/internal/shared/httputil"
)

type queryPatternsRequest struct {
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	Limit     int    `json:"limit"`
	Cursor    string `json:"cursor"`
	TenantID  int64  `json:"-"`

	filter.ExplorerFilters
}

func (r *queryPatternsRequest) BindTenant(tenantID int64) error {
	r.TenantID = tenantID
	if err := filterutil.ValidateTimeRange(&r.StartTime, &r.EndTime); err != nil {
		return err
	}
	if r.EndTime-r.StartTime > filterutil.RawRetentionMs {
		return errors.New("filters: span data is retained for 15 days")
	}
	return nil
}

func (h *Handler) QueryPatterns(w http.ResponseWriter, r *http.Request) {
	var req queryPatternsRequest
	if !modulecommon.BindFiltered(w, r, &req) {
		return
	}
	resp, err := h.Service.QueryPatterns(
		r.Context(),
		req.TenantID,
		req.StartTime,
		req.EndTime,
		req.ExplorerFilters,
		req.Limit,
		req.Cursor,
	)
	if err != nil {
		modulecommon.RespondErrorWithCause(
			w,
			r,
			http.StatusInternalServerError,
			errorcode.Internal,
			"Failed to query database patterns",
			err,
		)
		return
	}
	modulecommon.RespondOK(w, resp)
}
