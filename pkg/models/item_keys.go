package models

type ItemKey interface{}

type ScopedItemKey struct {
	Name  string
	Scope string
}

type BaseModelKey struct {
	Name    string
	Version string
	Type    string
}
