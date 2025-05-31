package models

import (
	"fmt"
)

// Environment represents a deployment environment.
type Environment struct {
	Type   string
	Region string
	Realm  string
}

// GetName returns the name of the environment.
func (e Environment) GetName() string {
	return fmt.Sprintf("%s-%s", e.Type, Region(e.Region).GetCode())
}

// GetFilterableFields returns filterable fields for the environment.
func (e Environment) GetFilterableFields() []string {
	return []string{e.Type, e.Region, e.Realm, e.GetName()}
}

// Equals returns true if the environment is equal to another environment.
func (e Environment) Equals(o Environment) bool {
	return e.Realm == o.Realm && e.GetName() == o.GetName()
}

// GetKey returns the key of the environment.
func (e Environment) GetKey() string {
	return fmt.Sprintf("%s-%s-%s", e.Realm, e.Type, Region(e.Region).GetCode())
}

// GetKubeContext returns the Kubernetes context string for the environment.
func (e Environment) GetKubeContext() string {
	envType := e.Type
	if envType == "preprod" {
		envType = "ppe"
	}

	return fmt.Sprintf("dp-%s-%s", envType, Region(e.Region).GetCode())
}
