package mutation

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Resolved is what a mutation's target lookup found — the node's
// instance, the pool's sizes, the InferenceService's namespace — which a
// dry run reports: MCP as its `resolved` object, the CLI after its
// DRY-RUN line. The constructors keep the keys the same on both.
type Resolved map[string]any

// ResolvedNode is a GPU node's lookup: the instance the action targets.
func ResolvedNode(instanceID string) Resolved { return Resolved{"instance_id": instanceID} }

// ResolvedPool is a GPU pool's lookup: its OCI instance pool, the size
// Terraform declares and the size it has.
func ResolvedPool(id string, size, actualSize int) Resolved {
	return Resolved{"pool_id": id, "size": size, "actual_size": actualSize}
}

// ResolvedNamespace is an InferenceService's lookup: its namespace.
func ResolvedNamespace(namespace string) Resolved { return Resolved{"namespace": namespace} }

// String renders r as "key=value, ..." in key order, "" when empty.
func (r Resolved) String() string {
	parts := make([]string, 0, len(r))
	for _, k := range slices.Sorted(maps.Keys(r)) {
		parts = append(parts, fmt.Sprintf("%s=%v", k, r[k]))
	}
	return strings.Join(parts, ", ")
}
