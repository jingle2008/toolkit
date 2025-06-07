package domain

// ToolkitContext holds the current scope and name for filtering or scoping operations.
type ToolkitContext struct {
	Category Category
	Name     string
}
