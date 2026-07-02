package filter

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const maxTimeRangeMs = 30 * 24 * 60 * 60 * 1000

type Filters struct {
	TeamID  int64 `json:"-"`
	StartMs int64 `json:"-"`
	EndMs   int64 `json:"-"`

	Services     []string `json:"services,omitempty"`
	Hosts        []string `json:"hosts,omitempty"`
	Pods         []string `json:"pods,omitempty"`
	Containers   []string `json:"containers,omitempty"`
	Environments []string `json:"environments,omitempty"`
	Severities   []string `json:"severities,omitempty"`

	TraceID    string `json:"traceId,omitempty"`
	SpanID     string `json:"spanId,omitempty"`
	Search     string `json:"search,omitempty"`
	SearchMode string `json:"searchMode,omitempty"`

	ExcludeServices   []string `json:"excludeServices,omitempty"`
	ExcludeHosts      []string `json:"excludeHosts,omitempty"`
	ExcludeSeverities []string `json:"excludeSeverities,omitempty"`

	Attributes []AttrFilter `json:"attributes,omitempty"`
}

type AttrFilter struct {
	Key   string `json:"key"`
	Op    string `json:"op,omitempty"`
	Value string `json:"value"`
}

func (f *Filters) Validate() error {
	if f.EndMs <= 0 {
		f.EndMs = time.Now().UnixMilli()
	}
	if f.StartMs <= 0 {
		return errors.New("filters: startTime is required")
	}
	if f.EndMs <= f.StartMs {
		return errors.New("filters: endTime must be after startTime")
	}
	if (f.EndMs - f.StartMs) > maxTimeRangeMs {
		f.StartMs = f.EndMs - maxTimeRangeMs
	}
	if strings.TrimSpace(f.SearchMode) == "" {
		f.SearchMode = "ngram"
	}
	return nil
}

// BuildFingerprintCTE turns a resource filter into a fingerprint-pruning CTE.
// It resolves matching fingerprints from the small logs_resource dimension table
// so the logs table scan can be pruned by its primary key. Both return values are
// empty when there is no resource filter.
func BuildFingerprintCTE(resourceWhere string) (cte, prewhereFP string) {
	if resourceWhere == "" {
		return "", ""
	}
	cte = `
		WITH active_fps AS (
		    SELECT DISTINCT fingerprint
		    FROM optikk.logs_resource
		    PREWHERE team_id = @teamID` + resourceWhere + `
		)`
	prewhereFP = " AND fingerprint IN active_fps"
	return cte, prewhereFP
}

func BuildClauses(f Filters) (resourceWhere, where string, args []any) {
	startBucket := uint32((f.StartMs / 1000) / 300 * 300)
	endBucket := uint32((f.EndMs / 1000) / 300 * 300)

	args = []any{
		clickhouse.Named("teamID", uint32(f.TeamID)),
		clickhouse.Named("start", time.UnixMilli(f.StartMs)),
		clickhouse.Named("end", time.UnixMilli(f.EndMs)),
		clickhouse.Named("startBucket", startBucket),
		clickhouse.Named("endBucket", endBucket),
	}

	resourceWhere += ` AND ts_bucket BETWEEN @startBucket AND @endBucket`
	where += ` AND ts_bucket BETWEEN @startBucket AND @endBucket`

	if len(f.Services) > 0 {
		resourceWhere += ` AND service IN @services`
		args = append(args, clickhouse.Named("services", f.Services))
	}
	if len(f.ExcludeServices) > 0 {
		resourceWhere += ` AND service NOT IN @excServices`
		args = append(args, clickhouse.Named("excServices", f.ExcludeServices))
	}
	if len(f.Hosts) > 0 {
		resourceWhere += ` AND host IN @hosts`
		args = append(args, clickhouse.Named("hosts", f.Hosts))
	}
	if len(f.ExcludeHosts) > 0 {
		resourceWhere += ` AND host NOT IN @excHosts`
		args = append(args, clickhouse.Named("excHosts", f.ExcludeHosts))
	}
	if len(f.Pods) > 0 {
		resourceWhere += ` AND pod IN @pods`
		args = append(args, clickhouse.Named("pods", f.Pods))
	}
	if len(f.Containers) > 0 {
		resourceWhere += ` AND container IN @containers`
		args = append(args, clickhouse.Named("containers", f.Containers))
	}
	if len(f.Environments) > 0 {
		resourceWhere += ` AND environment IN @environments`
		args = append(args, clickhouse.Named("environments", f.Environments))
	}

	if len(f.Severities) > 0 {
		where += ` AND severity_text IN @severities`
		args = append(args, clickhouse.Named("severities", f.Severities))
	}
	if len(f.ExcludeSeverities) > 0 {
		where += ` AND severity_text NOT IN @excSeverities`
		args = append(args, clickhouse.Named("excSeverities", f.ExcludeSeverities))
	}
	if f.TraceID != "" {
		where += ` AND trace_id = @traceID`
		args = append(args, clickhouse.Named("traceID", f.TraceID))
	}
	if f.SpanID != "" {
		where += ` AND span_id = @spanID`
		args = append(args, clickhouse.Named("spanID", f.SpanID))
	}
	if f.Search != "" {
		if f.SearchMode == "exact" {
			where += ` AND lower(body) LIKE concat('%', lower(@search), '%')`
		} else {
			where += ` AND hasToken(body, lower(@search))`
		}
		args = append(args, clickhouse.Named("search", f.Search))
	}
	for i, af := range f.Attributes {
		idx := strconv.Itoa(i)
		kName := "akey_" + idx
		vName := "aval_" + idx

		mapCol := "attributes_string"
		var val any = af.Value

		if af.Op == "" || af.Op == "neq" {
			if n, err := strconv.ParseFloat(af.Value, 64); err == nil {
				mapCol = "attributes_number"
				val = n
			} else if b, err := strconv.ParseBool(af.Value); err == nil {
				mapCol = "attributes_bool"
				val = b
			}
		}

		switch af.Op {
		case "neq":
			where += ` AND ` + mapCol + `[@` + kName + `] != @` + vName
		case "contains":
			where += ` AND positionCaseInsensitive(attributes_string[@` + kName + `], @` + vName + `) > 0`
		case "regex":
			where += ` AND match(attributes_string[@` + kName + `], @` + vName + `)`
		default:
			where += ` AND ` + mapCol + `[@` + kName + `] = @` + vName
		}
		args = append(args, clickhouse.Named(kName, af.Key), clickhouse.Named(vName, val))
	}
	return resourceWhere, where, args
}
