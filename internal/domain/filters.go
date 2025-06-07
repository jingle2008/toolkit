package domain

import (
	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/pkg/models"
)

// FilterByFilterable returns a filtered slice of items matching the provided filter string.
// T must implement models.NamedFilterable.
func FilterByFilterable[T models.NamedFilterable](items []T, filter string) []T {
	results := make([]T, 0, len(items))
	collections.FilterSlice(items, nil, filter, func(_ int, val T) bool {
		results = append(results, val)
		return true
	})
	return results
}

// FilterTenants returns a filtered slice of Tenant matching the provided filter string.
func FilterTenants(tenants []models.Tenant, filter string) []models.Tenant {
	return FilterByFilterable(tenants, filter)
}

func FilterServiceTenancies(tenancies []models.ServiceTenancy, filter string) []models.ServiceTenancy {
	return FilterByFilterable(tenancies, filter)
}

func FilterEnvironments(envs []models.Environment, filter string) []models.Environment {
	return FilterByFilterable(envs, filter)
}
