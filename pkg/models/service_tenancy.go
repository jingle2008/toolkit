package models

import "fmt"

// ServiceTenancy represents a service tenancy entity.
type ServiceTenancy struct {
	Name        string   `json:"tenancy_name"`
	Realm       string   `json:"realm"`
	HomeRegion  string   `json:"home_region"`
	Regions     []string `json:"regions"`
	Environment string   `json:"environment"`
}

// GetName returns the name of the service tenancy.
func (t ServiceTenancy) GetName() string {
	return t.Name
}

// GetFilterableFields returns filterable fields for the service tenancy.
func (t ServiceTenancy) GetFilterableFields() []string {
	return append(t.Regions[:], t.Name, t.Realm, t.HomeRegion, t.Environment)
}

// Environments returns the environments for the service tenancy.
func (t ServiceTenancy) Environments() []Environment {
	environments := make([]Environment, 0, len(t.Regions))
	for _, region := range t.Regions {
		env := Environment{
			Type:   t.Environment,
			Region: region,
			Realm:  t.Realm,
		}
		environments = append(environments, env)
	}

	return environments
}

// GetKey returns the key of the service tenancy.
func (t ServiceTenancy) GetKey() string {
	return fmt.Sprintf("%s-%s-%s", t.Realm, t.Environment, t.Name)
}
