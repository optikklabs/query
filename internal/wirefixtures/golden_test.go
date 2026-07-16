package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden wire file from the current response structs")

const goldenPath = "testdata/wire.golden.json"

// webFixturePath is where the web client keeps its copy of these fixtures.
// It is a sibling checkout, so this is a convenience for local regeneration
// only — CI checks out one repo at a time and will simply skip the copy.
const webFixturePath = "../../../web/src/shared/api/utils/__fixtures__.json"

const regenHint = `
The JSON shape of a query API response struct changed.

That is not automatically a bug, but it is always a contract change: the web
client hand-maintains Zod schemas against these exact bytes, and an added,
renamed, removed or retyped field can break it silently at runtime.

To resolve:

  1. go test ./internal/wirefixtures -update
  2. git diff -- internal/wirefixtures/testdata   # read what actually moved
  3. If a field was renamed, removed or retyped, update web's Zod schemas in
     web/src/shared/api/ to match, and copy the fixtures across:
       go run ./internal/wirefixtures > ../web/src/shared/api/utils/__fixtures__.json
       cd ../web && npx biome format --write src/shared/api/utils/__fixtures__.json && yarn test
`

// TestWireGolden pins the JSON encoding of every response struct the web client
// parses. It fails on any change to those structs by design: the golden diff is
// the review surface for a cross-repo contract change that nothing else in
// query's CI can see.
func TestWireGolden(t *testing.T) {
	got, err := encodeFixtures()
	if err != nil {
		t.Fatalf("encode fixtures: %v", err)
	}

	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("create testdata dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("wrote %s — now sync web's fixtures and Zod schemas%s", goldenPath, regenHint)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update to create it): %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("wire shape does not match %s%s", goldenPath, regenHint)
	}
}

// TestWebFixturesInSync catches the copy of these fixtures inside the web repo
// drifting from the golden. It only runs when web is checked out beside query,
// so it is a local safety net rather than a CI gate — query's CI has no access
// to the web repo. Whitespace is normalised because web reformats the file
// with biome.
func TestWebFixturesInSync(t *testing.T) {
	web, err := os.ReadFile(webFixturePath)
	if err != nil {
		t.Skipf("web checkout not present beside query (%v); skipping cross-repo sync check", err)
	}

	got, err := encodeFixtures()
	if err != nil {
		t.Fatalf("encode fixtures: %v", err)
	}

	if !bytes.Equal(canonicalJSON(t, got), canonicalJSON(t, web)) {
		t.Errorf("web's %s is stale relative to the Go response structs%s", filepath.Base(webFixturePath), regenHint)
	}
}

// canonicalJSON re-encodes JSON so two files that differ only in indentation
// compare equal. web runs biome over its copy, so byte equality is too strict.
func canonicalJSON(t *testing.T, raw []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parse json: %v", err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("re-encode json: %v", err)
	}
	return out
}
