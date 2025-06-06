package rows

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
)

/*
GetServiceTenancies returns table rows for a slice of ServiceTenancy, filtered by the provided filter string.
*/
func GetServiceTenancies(tenancies []models.ServiceTenancy, filter string) []table.Row {
	results := make([]table.Row, 0, len(tenancies))

	utils.FilterSlice(tenancies, nil, filter, func(_ int, val models.ServiceTenancy) bool {
		results = append(results, table.Row{
			val.Name,
			val.Realm,
			val.Environment,
			val.HomeRegion,
			strings.Join(val.Regions, ", "),
		})
		return true
	})

	return results
}
