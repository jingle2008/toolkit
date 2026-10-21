package models

import (
	"strconv"
	"strings"
)

// GPUPool represents a pool of GPUs.
type GPUPool struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Shape              string `json:"shape"`
	Size               int    `json:"size"`
	ActualSize         int    `json:"actualSize"`
	Status             string `json:"status"`
	IsOkeManaged       bool   `json:"isOkeManaged"`
	CapacityType       string `json:"capacityType"`
	AvailabilityDomain string `json:"availabilityDomain"`
}

// GPU pool statuses toolkit sets itself; OCI's lifecycle states (e.g.
// RUNNING) fill Status once the pool is enriched.
const (
	// GPUPoolStatusPending is a pool loaded from Terraform whose live state
	// hasn't been looked up yet.
	GPUPoolStatusPending = "..."
	// GPUPoolStatusUnknown is a pool whose live state couldn't be looked
	// up (the OCI call failed).
	GPUPoolStatusUnknown = "UNKNOWN"
	// GPUPoolStatusNonexistent is a pool Terraform declares that OCI has
	// no instance pool for.
	GPUPoolStatusNonexistent = "NONEXIST"
)

// GetName returns the name of the GPU pool.
func (p GPUPool) GetName() string {
	return p.Name
}

// FilterableFields returns filterable fields for the GPU pool.
func (p GPUPool) FilterableFields() []string {
	return []string{p.Name, p.Shape, p.CapacityType}
}

// IsFaulty reports whether the pool is short of its desired size. A pool
// scaled past it isn't: that is a deliberate over-scale (IsOverSize). A
// pool whose live state is pending or unknown isn't faulty either: its
// actual size is unknown, not zero. One OCI doesn't have is (its actual
// size is zero).
func (p GPUPool) IsFaulty() bool {
	return p.hasLiveSize() && p.ActualSize < p.Size
}

// IsOverSize reports whether the pool was scaled past its desired size,
// drift the next terraform apply undoes.
func (p GPUPool) IsOverSize() bool {
	return p.hasLiveSize() && p.ActualSize > p.Size
}

// hasLiveSize reports whether ActualSize was looked up.
func (p GPUPool) hasLiveSize() bool {
	return p.Status != GPUPoolStatusPending && p.Status != GPUPoolStatusUnknown
}

// Shortfall is how many instances the pool is short of its desired size,
// 0 when it is at or past it.
func (p GPUPool) Shortfall() int {
	return max(p.Size-p.ActualSize, 0)
}

// DefaultScaleAdd is how many instances a scale-up adds unless told
// otherwise: the shortfall, or one past the desired size.
func (p GPUPool) DefaultScaleAdd() int {
	if n := p.Shortfall(); n > 0 {
		return n
	}
	return 1
}

// GPUs returns the total number of GPUs in the pool.
func (p GPUPool) GPUs() int {
	parts := strings.Split(p.Shape, ".")
	count, _ := strconv.Atoi(parts[len(parts)-1])
	return count * p.Size
}
