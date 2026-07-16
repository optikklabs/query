// Command wirefixtures emits JSON exactly as the query API would, straight from
// the real response structs, so the web Zod schemas can be checked against the
// true wire shape instead of a hand-copy of it.
//
// The same fixtures are pinned by TestWireGolden, which fails whenever a
// response struct changes. See golden_test.go for the regeneration workflow.
package main

import (
	"fmt"
	"os"
)

func main() {
	data, err := encodeFixtures()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
