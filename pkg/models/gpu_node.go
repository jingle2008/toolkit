package models

import (
	"strconv"
	"strings"
)

type GpuNode struct {
	Name         string `json:"name"`
	InstanceType string `json:"instanceType"`
	NodePool     string `json:"poolName"`
	Allocatable  int    `json:"allocatable"`
	Allocated    int    `json:"allocated"`
	IsHealthy    bool   `json:"isHealthy"`
	IsReady      bool   `json:"isReady"`
}

func (n GpuNode) GetName() string {
	return n.Name
}

func (n GpuNode) GetFilterableFields() []string {
	return []string{n.Name, n.InstanceType, n.NodePool, n.GetStatus()}
}

func (n GpuNode) GetStatus() string {
	parts := strings.Split(n.InstanceType, ".")
	count, _ := strconv.Atoi(parts[len(parts)-1])
	if n.Allocatable != count {
		return "ERROR: Missing GPUs"
	} else if !n.IsHealthy {
		return "ERROR: Unhealthy"
	} else if !n.IsReady {
		return "ERROR: Not ready"
	}

	return "OK"
}
