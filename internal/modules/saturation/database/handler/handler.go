package handler

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/modules/saturation/database/repository"
	"github.com/optikklabs/query/internal/modules/saturation/database/service"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/filterutil"
	"github.com/optikklabs/query/internal/shared/httputil"
)

var queryHashPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

type Handler struct {
	service *service.Service
}

func New(databaseService *service.Service) *Handler {
	return &Handler{service: databaseService}
}

func (h *Handler) GetDatastoreSystems(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query datastore systems", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.service.GetDatastoreSystems(ctx, tenantID, startMs, endMs)
	})
}

func (h *Handler) GetLatencyBySystem(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query latency by system", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.service.GetLatencyBySystem(ctx, tenantID, startMs, endMs, filter.ParseFilters(r))
	})
}

func (h *Handler) GetOpsBySystem(w http.ResponseWriter, r *http.Request) {
	httputil.HandleRangeQuery(w, r, "Failed to query ops by system", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.service.GetOpsBySystem(ctx, tenantID, startMs, endMs, filter.ParseFilters(r))
	})
}

func parseHash(w http.ResponseWriter, r *http.Request) (string, bool) {
	values := r.URL.Query()["hash"]
	if len(values) != 1 || !queryHashPattern.MatchString(strings.TrimSpace(values[0])) {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "hash must be provided once as 16 lowercase hexadecimal characters", nil)
		return "", false
	}
	return values[0], true
}

func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	hash, ok := parseHash(w, r)
	if !ok {
		return
	}
	httputil.HandleRangeQuery(w, r, "Failed to query detail summary", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.service.GetSummary(ctx, tenantID, startMs, endMs, hash, filter.ParseFilters(r))
	})
}

func (h *Handler) GetTimeseries(w http.ResponseWriter, r *http.Request) {
	hash, ok := parseHash(w, r)
	if !ok {
		return
	}
	httputil.HandleRangeQuery(w, r, "Failed to query detail timeseries", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.service.GetTimeseries(ctx, tenantID, startMs, endMs, hash, filter.ParseFilters(r))
	})
}

func (h *Handler) GetExecutions(w http.ResponseWriter, r *http.Request) {
	hash, ok := parseHash(w, r)
	if !ok {
		return
	}
	httputil.HandleRangeQuery(w, r, "Failed to query detail executions", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.service.GetExecutions(ctx, tenantID, startMs, endMs, hash, filter.ParseFilters(r), httputil.ParsePageSize(r, "limit", service.DefaultExecutionsLimit))
	})
}

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
	return nil
}

func (h *Handler) QueryPatterns(w http.ResponseWriter, r *http.Request) {
	var req queryPatternsRequest
	if !httputil.BindFiltered(w, r, &req) {
		return
	}
	resp, err := h.service.QueryPatterns(
		r.Context(),
		req.TenantID,
		req.StartTime,
		req.EndTime,
		req.ExplorerFilters,
		req.Limit,
		req.Cursor,
	)
	if err != nil {
		httputil.RespondErrorWithCause(
			w,
			r,
			http.StatusInternalServerError,
			errorcode.Internal,
			"Failed to query database patterns",
			err,
		)
		return
	}
	httputil.RespondOK(w, resp)
}

func rejectUnknownQueryParams(w http.ResponseWriter, r *http.Request, allowed map[string]struct{}) bool {
	for key := range r.URL.Query() {
		if _, ok := allowed[key]; !ok {
			httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "unknown query parameter: "+key, nil)
			return true
		}
	}
	return false
}

func requiredSingleParam(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	values := r.URL.Query()[key]
	if len(values) != 1 || strings.TrimSpace(values[0]) == "" {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, key+" is required exactly once", nil)
		return "", false
	}
	return values[0], true
}

func requireExplicitRangeParams(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := requiredSingleParam(w, r, "startTime"); !ok {
		return false
	}
	if _, ok := requiredSingleParam(w, r, "endTime"); !ok {
		return false
	}
	return true
}

func (h *Handler) GetQueryPerformanceCatalogue(w http.ResponseWriter, r *http.Request) {
	allowed := map[string]struct{}{"startTime": {}, "endTime": {}, "dbSystem": {}}
	if rejectUnknownQueryParams(w, r, allowed) {
		return
	}
	if !requireExplicitRangeParams(w, r) {
		return
	}
	dbSystem, ok := requiredSingleParam(w, r, "dbSystem")
	if !ok {
		return
	}
	httputil.HandleRangeQuery(w, r, "Failed to query database query catalogue", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.service.GetQueryPerformanceCatalogue(ctx, tenantID, startMs, endMs, dbSystem)
	})
}

func parseSeriesScope(w http.ResponseWriter, r *http.Request) (collection, queryHash string, limit int, ok bool) {
	collectionValues := r.URL.Query()["collection"]
	queryHashValues := r.URL.Query()["queryHash"]
	if (len(collectionValues) == 1) == (len(queryHashValues) == 1) {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "exactly one of collection or queryHash is required", nil)
		return "", "", 0, false
	}
	if len(collectionValues) > 0 {
		if len(collectionValues) != 1 || strings.TrimSpace(collectionValues[0]) == "" {
			httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "collection must be a non-empty single value", nil)
			return "", "", 0, false
		}
		collection = collectionValues[0]
	}
	if len(queryHashValues) > 0 {
		if len(queryHashValues) != 1 || !queryHashPattern.MatchString(queryHashValues[0]) {
			httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "queryHash must be 16 lowercase hexadecimal characters", nil)
			return "", "", 0, false
		}
		queryHash = queryHashValues[0]
	}
	limit = repository.DefaultSeriesLimit
	limitValues := r.URL.Query()["limit"]
	if len(limitValues) > 1 {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "limit must be provided at most once", nil)
		return "", "", 0, false
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > repository.MaxSeriesLimit {
			httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "limit must be an integer between 1 and 100", nil)
			return "", "", 0, false
		}
		limit = parsed
	}
	return collection, queryHash, limit, true
}

func (h *Handler) GetQueryPerformanceSeries(w http.ResponseWriter, r *http.Request) {
	allowed := map[string]struct{}{
		"startTime": {}, "endTime": {}, "dbSystem": {},
		"collection": {}, "queryHash": {}, "limit": {},
	}
	if rejectUnknownQueryParams(w, r, allowed) {
		return
	}
	if !requireExplicitRangeParams(w, r) {
		return
	}
	dbSystem, ok := requiredSingleParam(w, r, "dbSystem")
	if !ok {
		return
	}
	collection, queryHash, limit, ok := parseSeriesScope(w, r)
	if !ok {
		return
	}
	httputil.HandleRangeQuery(w, r, "Failed to query database query series", func(ctx context.Context, tenantID, startMs, endMs int64) (any, error) {
		return h.service.GetQueryPerformanceSeries(ctx, tenantID, startMs, endMs, dbSystem, collection, queryHash, limit)
	})
}
