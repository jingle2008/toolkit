# GPU Workload category — design

**Date:** 2026-06-19
**Status:** Approved (pending spec review)

## Goal

Add a new TUI/CLI category, **`GPUWorkload`**, derived from Kubernetes pods that
consume GPU. Its parent is `GPUNode`, completing a three-level hierarchy:
`GPUPool → GPUNode → GPUWorkload`.

## Naming & detection

- **Name:** `GPUWorkload` (Go identifier and displayed category name, matching
  the existing `ImportedModel`/`DedicatedAICluster` no-space convention). Short
  alias `gw`.
- **Detection (broad):** a pod qualifies if the sum of its containers'
  `resources.limits["nvidia.com/gpu"]` is **> 0** and it has a non-empty
  `spec.nodeName` (scheduled). No serving/training label is required, so
  training/reservation/serving GPU pods all appear. `model`/`runtime`/`mode`
  are best-effort and render blank for non-serving pods.

## Fields extracted per pod

| Column   | Source                                            |
|----------|---------------------------------------------------|
| Name     | `metadata.name`                                   |
| Node     | `spec.nodeName` (also the group key)              |
| Tenant   | `labels["tenancy-id"]`, resolved to tenant Name   |
| Namespace| `metadata.namespace`                              |
| Model    | `labels["base-model-name"]`                       |
| Runtime  | `labels["serving-runtime"]`                       |
| GPUs     | Σ container `limits["nvidia.com/gpu"]` (int)      |
| Mode     | `annotations["ome.io/deploymentMode"]`            |

## Grouping & navigation

- Grouped by **`spec.nodeName`**, which equals `GPUNode.Name`.
- `GPUNode` becomes a scope category: `ScopedCategories(GPUNode) = [GPUWorkload]`,
  `GPUWorkload.Parents() = [GPUNode]`.
- Enter on a GPU node drills into its workloads; jump-to-parent returns to the
  node; view-details works — all via the existing grouped machinery, which
  requires the **group key (Node) at column index 1**.

## Column set (`internal/columns/gpu_workload.go`)

`GroupedSet`, ratios sum to 1.0, in order:
**Name(0), Node(1 = group key), Tenant(2), Namespace, Model, Runtime, GPUs, Mode.**
- Node column renders the group key.
- Tenant column renders the resolved tenant **Name** (fallback: raw `tenancy-id`
  suffix when unresolved); `RenderForExport` emits the full tenancy OCID.

## Touchpoints

1. **`internal/domain/category.go` (+ regenerated stringer)**
   - Add `GPUWorkload` const after `GPUNode`.
   - `ScopedCategories`: add `case GPUNode: return []Category{GPUWorkload}`.
   - `NeedsKubeConfig`: add `GPUWorkload → true`.
   - Add `gw` alias (mirrors the `gn`/`gp` short-alias handling).
   - Regenerate `category_string.go` via `go generate ./internal/domain`.

2. **`pkg/models/gpu_workload.go`**
   - `GPUWorkload{ Name, Node, TenantID, Namespace, Model, Runtime, GPUs int, Mode string, Owner *Tenant }`.
   - Implements `NamedFilterable` (`GetName`, `FilterableFields`, `IsFaulty`→false).
   - `TenancyOCID(realm)` and a tenant-name accessor for the Tenant column.

3. **`internal/infra/k8s/gpu_workload.go`**
   - `LoadGPUWorkloadsByNode(ctx, clientset) (map[string][]models.GPUWorkload, error)`.
   - Lists running pods; keeps those with GPU limit > 0 and a node; maps each to
     a `GPUWorkload`, grouped by `spec.nodeName`.
   - Helper `podGPULimits(pod) int` summing `limits["nvidia.com/gpu"]`.

4. **`internal/infra/loader/interfaces.go` + `production/production.go` + test doubles**
   - `GPUWorkloadLoader` added to `Composite`.
   - `production.Client.LoadGPUWorkloadsByNode` mirrors `LoadGPUNodesByPool`.
   - Update any fake/mock loaders implementing `Composite`.

5. **`pkg/models/dataset.go`**
   - `GPUWorkloadMap map[string][]GPUWorkload` field.
   - `SetGPUWorkloadMap(m)` resolving each item's `Owner` via the tenant-suffix
     map **without re-keying** (key stays the node).
   - Nil it in the realm-scoped reset.

6. **`internal/columns/` registry** — register `GPUWorkloadColumns` as a grouped
   source scoped by `GPUNode`.

7. **TUI wiring**
   - `row_sources.go`: `groupedSource(GPUWorkloadColumns, domain.GPUNode, …GPUWorkloadMap)`.
   - `table_utils.go` `itemKeyFrom`: add `GPUWorkload` to the grouped
     `ScopedItemKey` case (view-details).
   - `messages.go`, `loader_cmd.go`, `route_list_loaded.go`, `model_reducer.go`,
     `model_update.go`, `reducer_category.go`: the load
     message/command/apply/dispatch chain, with a `GPUWorkloadMap == nil` load
     guard (mirrors `GPUNode`).
   - `keys/registry.go`: `catContext[GPUWorkload] = {Parent, SortTenant, SortGpus, Refresh}`
     (+ global Name). Add a `SortGpus` (`G`) binding and `GpusCol` to the
     int-sort set in `table_sort.go`.

8. **CLI/MCP parity** — add `GPUWorkload` to the `get`/`resolve`/MCP load
   dispatch so `toolkit get gw` works (read-only; no delete/edit).

9. **Read-only:** no delete/edit/faulty wiring (`update_list_ops`, delete paths).

## Tests

- K8s loader extraction test using the provided sample pod (verifies all eight
  fields and node grouping; plus a non-GPU pod excluded and a non-serving GPU
  pod with blank model/runtime included).
- `domain` updates: `ScopedCategories(GPUNode)`, `Parents(GPUWorkload)`,
  `IsScope(GPUNode)`.
- Column-render test (field → cell mapping, ratio sum, Node at index 1).
- Grouped-key invariant entry for `GPUWorkload` (Node at row[1]) in the existing
  `TestGroupKeyAtRowIndex1`.

## Out of scope

- Editing/deleting workloads.
- Live pod status/health columns (phase, restarts) — only the eight fields above.
- Narrowing detection to serving-only pods (explicitly chose the broad filter).
