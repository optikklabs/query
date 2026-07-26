package app

import (
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/config"
	"github.com/optikklabs/query/internal/infra/token"
)

var updateRoutes = flag.Bool("update-routes", false, "rewrite the golden route table")

const routesGoldenPath = "testdata/routes.golden.txt"

// TestRoutesGolden pins the full HTTP route table. It exists to make module
// refactors provable: moving a handler between packages must not add, remove
// or rename a single route, and this is the only check in query that can say
// so. A diff here is either an intentional API change or a refactor bug.
func TestRoutesGolden(t *testing.T) {
	got := routeTable(t)

	if *updateRoutes {
		if err := os.WriteFile(routesGoldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("wrote %s", routesGoldenPath)
		return
	}

	want, err := os.ReadFile(routesGoldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update-routes to create it): %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("route table changed relative to %s.\n"+
			"If this is an intentional API change, re-run with -update-routes and\n"+
			"review the diff. If you are mid-refactor, this is a bug: a package\n"+
			"move must leave the route table byte-identical.", routesGoldenPath)
	}
}

// routeTable builds the real router and walks it. Modules are constructed with
// nil database handles: route registration never touches a connection, so this
// stays a pure unit test rather than requiring live MySQL and ClickHouse.
func routeTable(t *testing.T) []byte {
	t.Helper()

	cfg := config.Config{}
	infra := &Infra{Config: cfg, Tokens: token.NewService(cfg)}

	app := &App{
		Config:  cfg,
		Infra:   infra,
		Modules: configuredModules(nil, cfg, infra),
	}

	router, ok := app.Router().(chi.Routes)
	if !ok {
		t.Fatal("router does not implement chi.Routes")
	}

	var lines []string
	walk := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		lines = append(lines, fmt.Sprintf("%-7s %s", method, route))
		return nil
	}
	if err := chi.Walk(router, walk); err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	sort.Strings(lines)

	var buf bytes.Buffer
	for _, line := range lines {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}
