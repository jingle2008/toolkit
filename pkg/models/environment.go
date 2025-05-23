package models

import (
	"fmt"
)

type Environment struct {
	Type   string
	Region string
	Realm  string
}

func (e Environment) GetName() string {
	return fmt.Sprintf("%s-%s", e.Type, Region(e.Region).GetCode())
}

func (e Environment) GetFilterableFields() []string {
	return []string{e.Type, e.Region, e.Realm, e.GetName()}
}

func (e Environment) Equals(o Environment) bool {
	return e.Realm == o.Realm && e.GetName() == o.GetName()
}

func (e Environment) GetKey() string {
	return fmt.Sprintf("%s-%s-%s", e.Realm, e.Type, Region(e.Region).GetCode())
}

func (e Environment) GetKubeContext() string {
	envType := e.Type
	if envType == "preprod" {
		envType = "ppe"
	}

	return fmt.Sprintf("dp-%s-%s", envType, Region(e.Region).GetCode())
}
