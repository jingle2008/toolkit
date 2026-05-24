package models

// ImportedModel is a tenant-owned base model. Two sources feed this
// category:
//
//  1. Namespaced ome.io BaseModel CRs (across all namespaces) — the
//     `Namespace` field carries the originating namespace.
//  2. Cluster-scoped ClusterBaseModel CRs carrying a `tenancy-id`
//     label — `Namespace` is empty.
//
// Every item is grouped by tenant, matching the DedicatedAICluster
// pattern. Identity fields:
//
//   - `TenantID` is the OCI tenant identifier — the `tenancy-id`
//     label value, or `"UNKNOWN_TENANCY"` for orphans (namespaced
//     CRs missing the label, which is a config error). This is the
//     authoritative tenant key for grouping and lookups.
//   - `Owner` is a resolved pointer into Dataset.Tenants, set by
//     SetImportedModelMap when the OCID suffix matches a known
//     tenant. Nil for orphans or when the tenant isn't in the
//     realm's config. Same shape as DedicatedAICluster.Owner.
//   - `Namespace` is the K8s scope; empty for cluster-scoped CRs,
//     non-empty for namespaced CRs. Orthogonal to tenant identity
//     (a namespaced CR may carry a tenancy-id label that disagrees
//     with the namespace; we trust the label).
//
// Distinct from BaseModel (the shared / public catalog). The embedded
// BaseModel fields are JSON-inlined at the top level so consumers can
// reach `name`, `displayName`, `vendor`, etc. with the same paths
// they use for BaseModel; `namespace`, `tenantId`, and `owner` sit
// alongside.
type ImportedModel struct {
	BaseModel `yaml:",inline"`
	Namespace string  `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	TenantID  string  `json:"tenantId"            yaml:"tenantId"`
	Owner     *Tenant `json:"owner,omitempty"     yaml:"owner,omitempty"`
}

// GetFilterableFields extends BaseModel's filterable set with the
// imported-specific identity fields so `--filter namespace-x` or
// `--filter ocid1.tenancy.…` work without users knowing the source.
func (m ImportedModel) GetFilterableFields() []string {
	return append(m.BaseModel.GetFilterableFields(), m.Namespace, m.TenantID)
}
