package domain

import (
	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/pkg/models"
)

// FilterByFilterable returns a filtered slice of items matching the provided filter string.
// T must implement models.NamedFilterable.
func FilterByFilterable[T models.NamedFilterable](items []T, filter string) []T {
	return collections.Filter(items, func(val T) bool {
		return collections.IsMatch(val, filter, true)
	})
}

// FilterTenants returns a filtered slice of Tenant matching the provided filter string.
func FilterTenants(tenants []models.Tenant, filter string) []models.Tenant {
	return FilterByFilterable(tenants, filter)
}

/*
FilterServiceTenancies returns a filtered slice of ServiceTenancy matching the provided filter string.
*/
func FilterServiceTenancies(tenancies []models.ServiceTenancy, filter string) []models.ServiceTenancy {
	return FilterByFilterable(tenancies, filter)
}

/*
FilterEnvironments returns a filtered slice of Environment matching the provided filter string.
*/
func FilterEnvironments(envs []models.Environment, filter string) []models.Environment {
	return FilterByFilterable(envs, filter)
}
