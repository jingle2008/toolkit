package mutation

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

// Resolved is what a mutation's target lookup found — the node's
// instance, the pool's sizes, the InferenceService's namespace — which a
// dry run reports: MCP as its `resolved` object, the CLI after its
// DRY-RUN line. The constructors keep the keys the same on both.
type Resolved map[string]any

// ResolvedNode is a GPU node's lookup: the instance the action targets.
func ResolvedNode(instanceID string) Resolved { return Resolved{"instance_id": instanceID} }

// ResolvedPool is a GPU pool scale-up's lookup: its OCI instance pool,
// the size Terraform declares, the size it has, and the instances to add
// with the size that makes.
func ResolvedPool(id string, size, actualSize, add int) Resolved {
	return Resolved{"pool_id": id, "size": size, "actual_size": actualSize, "add": add, "target_size": actualSize + add}
}

// PoolOverSizeWarning is what every surface says before scaling a pool
// Terraform declares at declared instances to newSize, or "" when
// newSize doesn't pass it.
func PoolOverSizeWarning(declared, newSize int) string {
	if newSize <= declared {
		return ""
	}
	return fmt.Sprintf("Terraform declares %d; the next terraform apply shrinks it back and terminates the extra instances. Raise size in Terraform to keep them.", declared)
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

// PoolScaleAdd is how many instances the CLI and MCP add to pool: the
// requested count when given (at least 1), else the pool's shortfall —
// so a call without one still only fills the pool to its declared size,
// and 0 (nothing to do) for a pool already there. Going past the
// declared size always takes an explicit count.
func PoolScaleAdd(pool models.GPUPool, requested int, given bool) (int, error) {
	if !given {
		return pool.Shortfall(), nil
	}
	if requested < 1 {
		return 0, fmt.Errorf("add must be at least 1, got %d", requested)
	}
	return requested, nil
}

// PoolAtSizeNote is what the CLI and MCP say when a scale-up without a
// count finds pool already at its declared size; flag names the count
// on that surface ("--add", "add").
func PoolAtSizeNote(pool models.GPUPool, flag string) string {
	return fmt.Sprintf("%s is already at its Terraform size (%d); nothing to scale. Pass %s N to scale past it.", pool.Name, pool.Size, flag)
}

// PoolScaleNotice says what adding add instances to pool does, with the
// over-size warning when it passes the declared size. Sizes are from the
// last lookup; the scale re-reads OCI's size before it updates.
func PoolScaleNotice(pool models.GPUPool, add int) string {
	newSize := pool.ActualSize + add
	notice := fmt.Sprintf("Grows the instance pool from %d to %d instances.", pool.ActualSize, newSize)
	if w := PoolOverSizeWarning(pool.Size, newSize); w != "" {
		notice += "\n" + w
	}
	return notice
}
