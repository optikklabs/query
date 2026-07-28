package database

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/optikklabs/query/internal/infra/metrics"
)

var chGroup singleflight.Group

const leaderTimeout = 90 * time.Second

func coalesceKey(tenantID int64, query string, args []any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d\x00%s", tenantID, query)
	for _, a := range args {
		fmt.Fprintf(&b, "\x00%#v", a)
	}
	return b.String()
}

func coalesce(ctx context.Context, key, op string, dest any, fetch func(context.Context, any) error) error {
	destPtr := reflect.ValueOf(dest)
	if destPtr.Kind() != reflect.Pointer || destPtr.IsNil() {
		return fmt.Errorf("coalesce: dest must be a non-nil pointer, got %T", dest)
	}
	elemType := destPtr.Type().Elem()

	leader := false
	result := chGroup.DoChan(key, func() (any, error) {
		leader = true

		runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), leaderTimeout)
		defer cancel()

		fresh := reflect.New(elemType)
		if err := fetch(runCtx, fresh.Interface()); err != nil {
			return nil, err
		}
		return fresh.Elem().Interface(), nil
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-result:
		if !leader {
			metrics.DBQueriesCoalesced.WithLabelValues(op).Inc()
		}
		if res.Err != nil {
			return res.Err
		}
		assign(destPtr.Elem(), reflect.ValueOf(res.Val))
		return nil
	}
}

func assign(dst, src reflect.Value) {
	if src.Kind() == reflect.Slice && !src.IsNil() {
		out := reflect.MakeSlice(src.Type(), src.Len(), src.Len())
		reflect.Copy(out, src)
		dst.Set(out)
		return
	}
	dst.Set(src)
}
