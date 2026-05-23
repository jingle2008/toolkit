package models

// ImportedModel is a tenant-registered base model. Two sources feed
// this category:
//
//  1. Namespaced ome.io BaseModel CRs (across all namespaces) — the
//     `Namespace` field carries the originating namespace.
//  2. Cluster-scoped ClusterBaseModel CRs carrying a `tenancy-id`
//     label — the `Namespace` field is empty; `TenantID` carries the
//     label value.
//
// Distinct from BaseModel (the shared / public catalog). The embedded
// BaseModel fields are JSON-inlined at the top level so consumers can
// reach `name`, `displayName`, `vendor`, etc. with the same paths
// they use for BaseModel; `namespace`, `tenantId`, and `source` sit
// alongside.
type ImportedModel struct {
	BaseModel `yaml:",inline"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	TenantID  string `json:"tenantId,omitempty"  yaml:"tenantId,omitempty"`
	Source    string `json:"source"              yaml:"source"`
}

// Source values for ImportedModel.Source.
const (
	ImportedModelSourceNamespaced    = "namespaced"     // namespaced BaseModel CR
	ImportedModelSourceClusterScoped = "cluster-scoped" // ClusterBaseModel + tenancy-id label
)

// GetFilterableFields extends BaseModel's filterable set with the
// imported-specific identity fields so `--filter namespace-x` or
// `--filter ocid1.tenancy.…` work without users knowing the source.
func (m ImportedModel) GetFilterableFields() []string {
	return append(m.BaseModel.GetFilterableFields(), m.Namespace, m.TenantID, m.Source)
}
