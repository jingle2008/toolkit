package models

// ItemKey represents a generic item key.
type ItemKey any

// ScopedItemKey represents an item key with a scope.
type ScopedItemKey struct {
	Name  string
	Scope string
}

// BaseModelKey represents a key for a base model.
type BaseModelKey struct {
	Name    string
	Version string
	Type    string
}
