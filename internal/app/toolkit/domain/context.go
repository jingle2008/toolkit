package domain

// AppContext holds the current scope and name for filtering or scoping operations.
type AppContext struct {
	Category Category
	Name     string
}
