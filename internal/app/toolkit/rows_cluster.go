package toolkit

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/pkg/models"
)

/*
DedicatedAIClusterToRow adapts a models.DedicatedAICluster to a table.Row for display.
*/
func DedicatedAIClusterToRow(scope string, d models.DedicatedAICluster) table.Row {
	unitShapeOrProfile := d.UnitShape
	if unitShapeOrProfile == "" {
		unitShapeOrProfile = d.Profile
	}
	return table.Row{
		scope,
		d.Name,
		d.Type,
		unitShapeOrProfile,
		fmt.Sprint(d.Size),
		d.Status,
	}
}
