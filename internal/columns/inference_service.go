package columns

import (
	"github.com/jingle2008/toolkit/pkg/models"
)

// InferenceServiceColumns is the canonical column set for
// domain.InferenceService. 8 columns, ratios sum to 1.00. Namespace is the
// group key and MUST stay at index 1: itemKeyFrom derives the scoped key
// (under DedicatedAICluster) from row[1] for grouped categories. deploymentMode is omitted: it is RawDeployment everywhere.
// Type is omitted too: a dedicated row has a tenant and an amaaaaaa
// namespace, so the column only crowded the table. It stays in -o json
// and in filtering.
var InferenceServiceColumns = GroupedSet[models.InferenceService]{Columns: []GroupedColumn[models.InferenceService]{
	{
		Title: "Name", Key: "name", Ratio: 0.18,
		Render: func(_ string, s models.InferenceService) string { return s.Name },
	},
	{
		Title: "Namespace", Key: "namespace", Ratio: 0.18,
		Render: func(k string, _ models.InferenceService) string { return k },
	},
	{
		Title: "Tenant", Key: "tenant", Ratio: 0.13,
		Render: func(_ string, s models.InferenceService) string { return s.TenantName() },
		RenderForExport: func(realm, _ string, _ string, s models.InferenceService) string {
			return s.TenancyOCID(realm)
		},
	},
	{
		// Imported model names are OCID resource ids sharing a long
		// prefix, so truncate in the middle to keep the distinct tail.
		Title: "Model", Key: "model", Ratio: 0.17,
		Render: func(_ string, s models.InferenceService) string { return s.Model },
	},
	{
		Title: "Runtime", Key: "runtime", Ratio: 0.17,
		Render: func(_ string, s models.InferenceService) string { return s.Runtime },
	},
	{
		Title: "Replicas", Key: "replicas", Ratio: 0.07,
		Render: func(_ string, s models.InferenceService) string { return s.Replicas() },
	},
	{
		Title: "Ready", Key: "ready", Ratio: 0.06,
		Render: func(_ string, s models.InferenceService) string { return s.Ready },
	},
	{
		Title: "Age", Key: "age", Ratio: 0.04,
		Render: func(_ string, s models.InferenceService) string { return s.Age },
	},
}}
