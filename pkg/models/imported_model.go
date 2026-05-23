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
// `Namespace` and `TenantID` are orthogonal facets, not synonyms:
//
//   - `Namespace` is the K8s scope (empty ⇒ cluster-scoped CBM;
//     non-empty ⇒ namespaced BM). It's the authoritative
//     source-kind indicator.
//   - `TenantID` is the OCI tenant identifier, populated from the
//     `tenancy-id` label whenever present — including on namespaced
//     CRs that happen to carry the label. Matches the pattern used
//     by DedicatedAICluster (pkg/models/dedicated_ai_cluster.go).
//
// Both can be populated on the same item (namespaced CR with a
// tenancy-id label); neither implies the other. Consumers wanting
// "which K8s scope" should read `Namespace`; consumers wanting
// "which OCI tenant" should read `TenantID`.
//
// Distinct from BaseModel (the shared / public catalog). The embedded
// BaseModel fields are JSON-inlined at the top level so consumers can
// reach `name`, `displayName`, `vendor`, etc. with the same paths
// they use for BaseModel; `namespace` and `tenantId` sit alongside.
type ImportedModel struct {
	BaseModel `yaml:",inline"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	TenantID  string `json:"tenantId,omitempty"  yaml:"tenantId,omitempty"`
}

// GetFilterableFields extends BaseModel's filterable set with the
// imported-specific identity fields so `--filter namespace-x` or
// `--filter ocid1.tenancy.…` work without users knowing the source.
func (m ImportedModel) GetFilterableFields() []string {
	return append(m.BaseModel.GetFilterableFields(), m.Namespace, m.TenantID)
}
