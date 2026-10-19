package models

import (
	"fmt"
	"strings"
)

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
//   - `TenantID` is the `tenancy-id` label value (the OCID
//     short-name suffix, not the full OCID), or `"UNKNOWN_TENANCY"`
//     for orphans (namespaced CRs missing the label, which is a
//     config error). Same shape as DedicatedAICluster.TenantID. This
//     is the authoritative tenant key for grouping and lookups; use
//     GetTenantID(realm) to render the full OCID.
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
	BaseModel
	Namespace string `json:"namespace,omitempty"`
	// Copies lists the namespaces of further copies of the same model
	// (same name, same tenant), sorted. A tenant's model lives in
	// `default` and is copied into a DAC's namespace to be served there
	// (a namespaced model ref resolves in the endpoint's own namespace);
	// the copies are one model, so they are one row, with Namespace the
	// primary copy (`default` when present). Empty for most models.
	Copies   []string `json:"copies,omitempty"`
	TenantID string   `json:"tenantId"`
	Owner    *Tenant  `json:"owner,omitempty"`
}

// Namespaces returns the namespaces of every copy: Namespace, then Copies.
func (m ImportedModel) Namespaces() []string {
	if m.Namespace == "" && len(m.Copies) == 0 {
		return nil
	}
	return append([]string{m.Namespace}, m.Copies...)
}

// DACNamespaces returns the namespaces of the copies that sit in a DAC's
// namespace (named after the DAC's OCID) — one per dedicated AI cluster
// serving the model. Empty when the model isn't deployed on a DAC; more
// than one when several DACs serve it.
func (m ImportedModel) DACNamespaces() []string {
	var out []string
	for _, ns := range m.Namespaces() {
		if IsDACName(ns) {
			out = append(out, ns)
		}
	}
	return out
}

// FilterableFields extends BaseModel's filterable set with the
// imported-specific identity fields so `--filter namespace-x` or
// `--filter ocid1.tenancy.…` work without users knowing the source.
func (m ImportedModel) FilterableFields() []string {
	fields := append(m.BaseModel.FilterableFields(), m.Namespace, m.TenantID)
	return append(fields, m.Copies...)
}

// ociResourceIDPrefix leads the OCID resource ids that name an imported
// model and a DAC (and so the namespace named after a DAC). Shared-catalog
// model names never carry it (checked on dp-prod-ord: none of 200
// BaseModels does, all 253 ImportedModels do).
const ociResourceIDPrefix = "amaaaaaa"

// IsDACName reports whether name is a dedicated AI cluster's (an OCID
// resource id), and so whether a namespace is named after a DAC: a
// dedicated endpoint's and its pods', and an imported model copy's
// served there. A shared endpoint's namespace is its model's name.
func IsDACName(name string) bool {
	return strings.HasPrefix(name, ociResourceIDPrefix)
}

// IsImportedModelName reports whether a model name is an imported model's
// (an OCID resource id) rather than a shared-catalog name. This holds
// whichever way the model is referenced: a namespaced BaseModel, or a
// tenant-labelled ClusterBaseModel.
func IsImportedModelName(name string) bool {
	return strings.HasPrefix(name, ociResourceIDPrefix)
}

// TenancyOCID returns the full tenancy OCID for the ImportedModel by
// combining the realm with the `tenancy-id` label suffix stored in
// TenantID. Mirrors DedicatedAICluster.TenancyOCID.
func (m ImportedModel) TenancyOCID(realm string) string {
	return tenancyOCID(realm, m.TenantID)
}

// OwnerState returns the owner's internal/external state ("true" /
// "false"), or "" when the owning tenant is unresolved. Mirrors
// DedicatedAICluster.OwnerState.
func (m ImportedModel) OwnerState() string {
	if m.Owner != nil {
		return fmt.Sprint(m.Owner.IsInternal)
	}
	return ""
}

// OCID returns the full OCID for the ImportedModel by combining the
// realm and region with the Name suffix. Mirrors
// DedicatedAICluster.OCID; PHX/IAD regions are normalized to their
// short codes the same way.
func (m ImportedModel) OCID(realm, region string) string {
	region = normalizeRegion(region)
	return fmt.Sprintf("ocid1.generativeaiimportedmodel.%s.%s.%s", realm, region, m.Name)
}
