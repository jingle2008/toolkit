package domain

import (
	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/pkg/models"
)

// FilterTenants returns a filtered slice of Tenant matching the provided filter string.
func FilterTenants(tenants []models.Tenant, filter string) []models.Tenant {
	results := make([]models.Tenant, 0, len(tenants))

	collections.FilterSlice(tenants, nil, filter, func(_ int, val models.Tenant) bool {
		results = append(results, val)
		return true
	})

	return results
}

func FilterServiceTenancies(tenancies []models.ServiceTenancy, filter string) []models.ServiceTenancy {
	results := make([]models.ServiceTenancy, 0, len(tenancies))

	collections.FilterSlice(tenancies, nil, filter, func(_ int, val models.ServiceTenancy) bool {
		results = append(results, val)
		return true
	})

	return results
}

func FilterEnvironments(envs []models.Environment, filter string) []models.Environment {
	results := make([]models.Environment, 0, len(envs))

	collections.FilterSlice(envs, nil, filter, func(_ int, val models.Environment) bool {
		results = append(results, val)
		return true
	})

	return results
}
