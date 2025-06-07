package tenant

import (
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
)

// Filter returns a filtered slice of Tenant matching the provided filter string.
func Filter(tenants []models.Tenant, filter string) []models.Tenant {
	results := make([]models.Tenant, 0, len(tenants))

	utils.FilterSlice(tenants, nil, filter, func(_ int, val models.Tenant) bool {
		results = append(results, val)
		return true
	})

	return results
}
