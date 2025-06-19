package models

// TenancyOverrideGroup bundles tenants and their three override maps.
type TenancyOverrideGroup struct {
	Tenants                           []Tenant
	LimitTenancyOverrideMap           map[string][]LimitTenancyOverride
	ConsolePropertyTenancyOverrideMap map[string][]ConsolePropertyTenancyOverride
	PropertyTenancyOverrideMap        map[string][]PropertyTenancyOverride
}
