package toolkit

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/pkg/models"
)

/*
gpuNodeRow is a wrapper to implement RowMarshaler for models.GpuNode.
*/
type gpuNodeRow models.GpuNode

func (n gpuNodeRow) ToRow(_ string) table.Row {
	return table.Row{
		n.NodePool,
		n.Name,
		n.InstanceType,
		fmt.Sprint(n.Allocatable),
		fmt.Sprint(n.Allocatable - n.Allocated),
		fmt.Sprint(n.IsHealthy),
		fmt.Sprint(n.IsReady),
		models.GpuNode(n).GetStatus(),
	}
}
