package toolkit

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/pkg/models"
)

/*
GpuNodeToRow adapts a models.GpuNode to a table.Row for display.
*/
func GpuNodeToRow(_ string, n models.GpuNode) table.Row {
	return table.Row{
		n.NodePool,
		n.Name,
		n.InstanceType,
		fmt.Sprint(n.Allocatable),
		fmt.Sprint(n.Allocatable - n.Allocated),
		fmt.Sprint(n.IsHealthy),
		fmt.Sprint(n.IsReady),
		n.GetStatus(),
	}
}
