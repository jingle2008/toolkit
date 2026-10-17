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

// IsFaulty reports whether the pool's actual size differs from its desired
// size. A pool whose live state is pending or unknown isn't faulty: its
// actual size is unknown, not zero. One OCI doesn't have is (its actual
// size is zero).
func (p GPUPool) IsFaulty() bool {
	if p.Status == GPUPoolStatusPending || p.Status == GPUPoolStatusUnknown {
		return false
	}
	return p.ActualSize != p.Size
}

// GPUs returns the total number of GPUs in the pool.
func (p GPUPool) GPUs() int {
	parts := strings.Split(p.Shape, ".")
	count, _ := strconv.Atoi(parts[len(parts)-1])
	return count * p.Size
}
