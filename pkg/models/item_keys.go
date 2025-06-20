package models

// ItemKey represents a generic item key.
type ItemKey any

// ScopedItemKey represents an item key with a scope.
type ScopedItemKey struct {
	Name  string
	Scope string
}
