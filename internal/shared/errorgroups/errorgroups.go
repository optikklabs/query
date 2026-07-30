// Package errorgroups owns the ClickHouse projection that defines an error
// group's display identity. Keeping this fragment shared prevents error
// tracking and version comparisons from assigning different labels to the
// same error_group_id.
package errorgroups

import "strings"

const Predicate = "is_error = 1"

// QualifiedPredicate returns the canonical error-span predicate for a table
// alias. An empty alias returns Predicate unchanged.
func QualifiedPredicate(alias string) string {
	if alias == "" {
		return Predicate
	}
	return strings.TrimSuffix(alias, ".") + "." + Predicate
}

// IdentityProjection projects the stable group id and the latest human
// readable identity observed for that group. The query using this fragment
// must GROUP BY error_group_id.
func IdentityProjection(alias string) string {
	prefix := strings.TrimSuffix(alias, ".")
	if prefix != "" {
		prefix += "."
	}
	return prefix + `error_group_id AS error_group_id,
		       argMax(` + prefix + `name, (` + prefix + `timestamp, ` + prefix + `span_id)) AS operation_name,
		       argMax(` + prefix + `exception_type, (` + prefix + `timestamp, ` + prefix + `span_id)) AS exception_type`
}
