package monitors

import (
	"net/http"
	"time"

	httputil "github.com/optikklabs/query/internal/shared/httputil"
)

func (h *Handler) Ack(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.Service.Ack(r.Context(), tenant.TenantID, tenant.UserID, id); err != nil {
		httputil.RespondServiceError(w, r, err, "monitor request failed")
		return
	}
	httputil.RespondOK(w, map[string]any{"acked": id})
}

func (h *Handler) Mute(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	var req MuteRequest
	if !httputil.BindJSON(w, r, &req) {
		return
	}
	if err := h.Service.Mute(r.Context(), tenant.TenantID, id, req.DurationSec); err != nil {
		httputil.RespondServiceError(w, r, err, "monitor request failed")
		return
	}
	httputil.RespondOK(w, map[string]any{"muted": id, "durationSec": req.DurationSec})
}

func (h *Handler) Unmute(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	if err := h.Service.Unmute(r.Context(), tenant.TenantID, id); err != nil {
		httputil.RespondServiceError(w, r, err, "monitor request failed")
		return
	}
	httputil.RespondOK(w, map[string]any{"unmuted": id})
}

func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	res, err := h.Service.Test(r.Context(), tenant.TenantID, id, h.Queries)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "monitor request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Series(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	windowMs, ok := httputil.QueryInt64(w, r, "windowMs", time.Hour.Milliseconds())
	if !ok {
		return
	}
	res, err := h.Service.Series(r.Context(), tenant.TenantID, id, h.Queries, windowMs)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "monitor request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	limit, ok := httputil.QueryLimit(w, r, defaultEventLimit, maxListLimit)
	if !ok {
		return
	}
	res, err := h.Service.Events(r.Context(), tenant.TenantID, id, limit)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "monitor request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) Activity(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	since, ok := httputil.QueryInt64(w, r, "since", 0)
	if !ok {
		return
	}
	limit, ok := httputil.QueryLimit(w, r, defaultEventLimit, maxListLimit)
	if !ok {
		return
	}
	res, err := h.Service.Activity(r.Context(), tenant.TenantID, since, limit)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "monitor request failed")
		return
	}
	httputil.RespondOK(w, res)
}

func (h *Handler) StatusTimeline(w http.ResponseWriter, r *http.Request) {
	tenant := httputil.Tenant(r)
	id, ok := httputil.ParseIDParam(w, r, "id")
	if !ok {
		return
	}
	windowMs, ok := httputil.QueryInt64(w, r, "windowMs", (24 * time.Hour).Milliseconds())
	if !ok {
		return
	}
	res, err := h.Service.StatusTimeline(r.Context(), tenant.TenantID, id, windowMs)
	if err != nil {
		httputil.RespondServiceError(w, r, err, "monitor request failed")
		return
	}
	httputil.RespondOK(w, res)
}
