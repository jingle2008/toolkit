package models

import (
	"fmt"
	"strings"
)

/*
ResolveEnvType canonicalizes a user-supplied environment type.

The input is lowercased and trimmed, and "ppe" resolves to "preprod" —
the spelling the shep_targets data uses, and therefore the only one
that matches during environment validation. The opposite direction
already exists on the way out: KubeContext and the GenAI endpoint
prefix both rewrite "preprod" to "ppe".

Anything else is returned as-is. Unlike regions there is no closed set
to check against: valid types are whatever the repo's shep_targets
entries declare, so an unknown type is left for the loader to reject.
*/
func ResolveEnvType(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "ppe" {
		return "preprod"
	}
	return v
}

// Environment represents a deployment environment.
type Environment struct {
	Type   string
	Region string
	Realm  string
}

// GetName returns the name of the environment.
func (e Environment) GetName() string {
	return fmt.Sprintf("%s-%s", e.Type, Region(e.Region).Code())
}

// FilterableFields returns filterable fields for the environment.
func (e Environment) FilterableFields() []string {
	return []string{e.Type, e.Region, e.Realm, e.GetName()}
}

// IsFaulty returns false by default for Environment.
func (e Environment) IsFaulty() bool {
	return false
}

// Equals returns true if the environment is equal to another environment.
func (e Environment) Equals(o Environment) bool {
	return e.Realm == o.Realm && e.GetName() == o.GetName()
}

// KubeContext returns the Kubernetes context string for the environment.
func (e Environment) KubeContext() string {
	envType := e.Type
	if envType == "preprod" {
		envType = "ppe"
	}

	return fmt.Sprintf("dp-%s-%s", envType, Region(e.Region).Code())
}
