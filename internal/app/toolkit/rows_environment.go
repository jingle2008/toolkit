package toolkit

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
)

// Adapter function for models.Environment
func getEnvironments(envs []models.Environment, filter string) []table.Row {
	results := make([]table.Row, 0, len(envs))

	utils.FilterSlice(envs, nil, filter, func(_ int, val models.Environment) bool {
		results = append(results, table.Row{
			val.GetName(),
			val.Realm,
			val.Type,
			val.Region,
		})
		return true
	})

	return results
}
