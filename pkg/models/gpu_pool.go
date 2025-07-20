package models

import (
	"strconv"
	"strings"
)

// GpuPool represents a pool of GPUs.
type GpuPool struct {
	Name         string
	Shape        string
	Size         int
	IsOkeManaged bool
	CapacityType string
}

// GetName returns the name of the GPU pool.
func (p GpuPool) GetName() string {
	return p.Name
}

// GetFilterableFields returns filterable fields for the GPU pool.
func (p GpuPool) GetFilterableFields() []string {
	return []string{p.Name, p.Shape, p.CapacityType}
}

// IsFaulty returns false by default for GpuPool.
func (p GpuPool) IsFaulty() bool {
	return false
}

// GetGPUs returns the total number of GPUs in the pool.
func (p GpuPool) GetGPUs() int {
	parts := strings.Split(p.Shape, ".")
	count, _ := strconv.Atoi(parts[len(parts)-1])
	return count * p.Size
}
