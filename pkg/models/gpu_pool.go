package models

import (
	"strconv"
	"strings"
)

type GpuPool struct {
	Name         string
	Shape        string
	Size         int
	IsOkeManaged bool
	CapacityType string
}

func (p GpuPool) GetName() string {
	return p.Name
}

func (p GpuPool) GetFilterableFields() []string {
	return []string{p.Name, p.Shape, p.CapacityType}
}

func (p GpuPool) GetGPUs() int {
	parts := strings.Split(p.Shape, ".")
	count, _ := strconv.Atoi(parts[len(parts)-1])
	return count * p.Size
}
