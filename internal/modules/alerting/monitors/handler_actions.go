package monitors

import (
	"errors"
	"net/http"

	"github.com/optikklabs/query/internal/shared/errorcode"
	httputil "github.com/optikklabs/query/internal/shared/httputil"
)

func (h *Handler) Ack(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.Service.Ack(r.Context(), tenant.TeamID, tenant.UserID, id); err != nil {
		if errors.Is(err, ErrNotAlerting) {
			httputil.RespondError(w, r, http.StatusConflict, errorcode.Conflict, "monitor is not currently alerting")
			return
		}
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"acked": id})
}

func (h *Handler) Mute(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req MuteRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.RespondError(w, r, http.StatusBadRequest, errorcode.Validation, err.Error())
		return
	}
	if err := h.Service.Mute(r.Context(), tenant.TeamID, id, req.DurationSec); err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"muted": id, "duration_sec": req.DurationSec})
}

func (h *Handler) Unmute(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.Service.Unmute(r.Context(), tenant.TeamID, id); err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, map[string]any{"unmuted": id})
}

func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	res, err := h.Service.Test(r.Context(), tenant.TeamID, id, h.Queries)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Series(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	windowMs := int64(httputil.ParseIntParam(r, "window_ms", 3_600_000))
	res, err := h.Service.Series(r.Context(), tenant.TeamID, id, h.Queries, windowMs)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	limit := httputil.ParseIntParam(r, "limit", 20)
	res, err := h.Service.Events(r.Context(), tenant.TeamID, id, limit)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Activity(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	since := httputil.ParseInt64Param(r, "since", 0)
	limit := httputil.ParseIntParam(r, "limit", 20)
	res, err := h.Service.Activity(r.Context(), tenant.TeamID, since, limit)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) StatusTimeline(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	windowMs := int64(httputil.ParseIntParam(r, "window_ms", 24*60*60*1000))
	res, err := h.Service.StatusTimeline(r.Context(), tenant.TeamID, id, windowMs)
	if err != nil {
		respondServiceError(w, r, err)
		return
	}
	httputil.RespondOK(w, res)
}
