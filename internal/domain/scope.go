package domain

// Scope is a (Category, Name) pair identifying a parent under which
// child rows are listed — e.g. Scope{Tenant, "MyTenant"} selects the
// DACs / ImportedModels owned by that tenant. Used by the TUI's
// table renderer to filter grouped categories and by the CLI to
// resolve --filter / --context flags.
type Scope struct {
	Category Category
	Name     string
}
