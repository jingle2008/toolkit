package rows

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
)

// Adapter function for models.Tenant
func GetTenants(tenants []models.Tenant, filter string) []table.Row {
	results := make([]table.Row, 0, len(tenants))

	utils.FilterSlice(tenants, nil, filter, func(_ int, val models.Tenant) bool {
		results = append(results, table.Row{
			val.Name,
			val.GetTenantID(),
			val.GetOverrides(),
		})
		return true
	})

	return results
}
