package rows

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/pkg/models"
)

/*
dedicatedAIClusterRow is a wrapper to implement RowMarshaler for models.DedicatedAICluster.
*/
type dedicatedAIClusterRow models.DedicatedAICluster

func (d dedicatedAIClusterRow) ToRow(scope string) table.Row {
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
