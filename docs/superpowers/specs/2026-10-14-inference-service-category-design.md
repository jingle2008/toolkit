# InferenceService category — design

**Date:** 2026-10-14
**Status:** Approved

## Goal

Add a TUI/CLI/MCP category, **`InferenceService`**, backed by OME
`InferenceService` CRs (`ome.io/v1beta1`, resource `inferenceservices`,
namespaced). Its parent is `DedicatedAICluster`, extending the tenant
hierarchy: `Tenant → DedicatedAICluster → InferenceService`.

Also fix Tab/Shift-Tab category cycling, which stopped at
`DedicatedAICluster` and skipped `ServingRuntime`.

## Cluster facts (dp-prod-ord, 149 objects)

- **Dedicated** (131): namespace = DAC ID (equals `DedicatedAICluster.Name`
  and the `ome.io/dedicated-ai-cluster` annotation), name = endpoint ID,
  `tenancy-id` label present.
- **On-demand** (18): namespace = name = model name, no `tenancy-id`, no
  DAC annotation.
- `spec.model` refs a `ClusterBaseModel` (125) or a namespaced `BaseModel`
  (24); `spec.runtime` refs a `ClusterServingRuntime` (absent on 24);
  `spec.engine.{min,max}Replicas`.
- `ome.io/deploymentMode` is `RawDeployment` everywhere — not surfaced.

## Naming

`InferenceService`, short alias `is` (the auto-computed capital-letter
alias; no extra alias).

## Grouping & navigation

- Grouped by **namespace**. `ScopedCategories(DedicatedAICluster) =
  [InferenceService]`, so Enter on a DAC drills into its inference
  services (details remain on the view-details key, as for GPUNode).
- On-demand endpoints appear only in the unscoped list.

## Columns (`GroupedSet`)

**Name(0), Namespace(1 = group key), Type, Tenant, Model, Runtime,
Replicas, Ready, Age.**

- Type: `dedicated` (namespace is a DAC ID) or `on-demand`.
- Tenant: resolved tenant name, raw `tenancy-id` fallback, blank when
  unlabelled. Export emits the full tenancy OCID.
- Model: `spec.model.name`, suffixed `*` when the ref kind is the
  namespaced `BaseModel` (tenant-imported).
- Runtime: `spec.runtime.name`, blank when unset.
- Replicas: `min/max` from `spec.engine`.
- Ready: the `Ready` condition status (`True`/`False`/`Unknown`).

## Behaviour

- `IsFaulty()` ⇔ Ready ≠ `True`.
- Read-only: no delete/scale/restart.
- Live watch on `inferenceservices` CRs.
- Keys: Parent, SortTenant, SortAge, CopyTenant, OpenMetrics,
  ToggleFaulty, Refresh.
- Metrics: dedicated rows use the DAC filter (namespace → DAC OCID) and
  the model; on-demand rows use the base-catalog plan for the model.
- CLI/MCP: `toolkit get is` and the MCP list tools.

## Tab cycling

Tab/Shift-Tab cycle every data category, `Tenant … ServingRuntime`
(Alias excluded), via a shared `domain.LastDataCategory` bound. A drift
test asserts the cycle visits every category except Alias exactly once.

## Out of scope

Cross-jumps (IS → model/runtime/pods) and mutations.
