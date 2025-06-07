package rows

import (
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
)

/*
ServiceTenancies returns a filtered slice of ServiceTenancy matching the provided filter string.
*/
func ServiceTenancies(tenancies []models.ServiceTenancy, filter string) []models.ServiceTenancy {
	results := make([]models.ServiceTenancy, 0, len(tenancies))

	utils.FilterSlice(tenancies, nil, filter, func(_ int, val models.ServiceTenancy) bool {
		results = append(results, val)
		return true
	})

	return results
}
