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

// GetName returns the name of the GPU pool.
func (p GPUPool) GetName() string {
	return p.Name
}

// GetFilterableFields returns filterable fields for the GPU pool.
func (p GPUPool) GetFilterableFields() []string {
	return []string{p.Name, p.Shape, p.CapacityType}
}

// IsFaulty reports whether the pool's actual size differs from its desired size.
func (p GPUPool) IsFaulty() bool {
	return p.ActualSize != p.Size
}

// GetGPUs returns the total number of GPUs in the pool.
func (p GPUPool) GetGPUs() int {
	parts := strings.Split(p.Shape, ".")
	count, _ := strconv.Atoi(parts[len(parts)-1])
	return count * p.Size
}
