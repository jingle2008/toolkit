# GPUWorkload Category Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `GPUWorkload` category (child of `GPUNode`) derived from GPU-consuming Kubernetes pods, viewable in the TUI and CLI.

**Architecture:** Mirror the existing grouped-category pattern (GPUNode/DedicatedAICluster). A k8s loader lists GPU pods and groups them by node name; the dataset resolves each item's owning tenant in place; columns/row-source/keys/load-wiring follow the established conventions. Hierarchy becomes GPUPool → GPUNode → GPUWorkload.

**Tech Stack:** Go, bubbletea TUI, client-go (typed clientset + fake), testify.

## Global Constraints

- Category name (Go identifier + displayed): `GPUWorkload`. Short alias: `gw`.
- Detection: a pod qualifies iff Σ container `limits["nvidia.com/gpu"]` > 0 AND `spec.nodeName` is non-empty.
- Group key = `spec.nodeName` (equals `GPUNode.Name`).
- Columns, in order: Name(0), Node(1=group key), Tenant(2), Namespace, Model, Runtime, GPUs, Mode. Ratios sum to 1.0.
- Tenant column: resolved tenant `Name` (fallback raw `tenancy-id` suffix); export = full tenancy OCID.
- Read-only category: no delete/edit/faulty wiring.
- After all tasks: run `go build ./...`, `go vet ./...`, `go test ./...` — all must pass.
- Commit after each task. Work on a branch `feat/gpu-workload-category` (not main).

---

### Task 1: Domain — add GPUWorkload category

**Files:**
- Modify: `internal/domain/category.go`
- Modify (generated): `internal/domain/category_string.go`
- Test: `internal/domain/category_test.go`

**Interfaces:**
- Produces: `domain.GPUWorkload` (Category const); `GPUNode.ScopedCategories() == [GPUWorkload]`; `GPUWorkload.Parents() == [GPUNode]`; `GPUWorkload.NeedsKubeConfig() == true`; alias `gw`.

- [ ] **Step 1: Write failing tests**

In `internal/domain/category_test.go`, extend `TestCategory_ScopedCategories` cases and `TestCategory_Parents` cases, and add a focused test:

```go
func TestCategory_GPUWorkload(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []Category{GPUWorkload}, GPUNode.ScopedCategories())
	assert.True(t, GPUNode.IsScope())
	assert.Equal(t, []Category{GPUNode}, GPUWorkload.Parents())
	assert.True(t, GPUWorkload.NeedsKubeConfig())
	c, err := ParseCategory("gw")
	require.NoError(t, err)
	assert.Equal(t, GPUWorkload, c)
}
```

(Add `"github.com/stretchr/testify/require"` to imports if not present.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/ -run TestCategory_GPUWorkload`
Expected: FAIL — `undefined: GPUWorkload`.

- [ ] **Step 3: Add the const**

In `internal/domain/category.go`, insert after the `GPUNode` const (keep the `DedicatedAICluster`/`Alias` consts after it):

```go
	// GPUNode is a category for GPU nodes.
	GPUNode
	// GPUWorkload is a category for GPU-consuming pods (any pod that
	// requests nvidia.com/gpu). Scoped by GPUNode: GPUPool → GPUNode →
	// GPUWorkload.
	GPUWorkload
	// DedicatedAICluster is a category for dedicated AI clusters.
	DedicatedAICluster
```

- [ ] **Step 4: Wire ScopedCategories, NeedsKubeConfig, alias**

In `ScopedCategories`, add a case:

```go
	case GPUPool:
		return []Category{GPUNode}
	case GPUNode:
		return []Category{GPUWorkload}
```

In `NeedsKubeConfig`, add `GPUWorkload`:

```go
	case BaseModel, ImportedModel, GPUNode, DedicatedAICluster, GPUWorkload:
		return true
```

In `Aliases`, add the short-alias case (mirrors `gn`/`gp`):

```go
	case DedicatedAICluster:
		aliases = append(aliases, "dac")
	case GPUWorkload:
		aliases = append(aliases, "gw")
```

- [ ] **Step 5: Regenerate the stringer**

Run: `go generate ./internal/domain/`
If `stringer` is missing: `go install golang.org/x/tools/cmd/stringer@latest` then re-run.
Expected: `internal/domain/category_string.go` now includes `GPUWorkload`.

- [ ] **Step 6: Run tests**

Run: `go test ./internal/domain/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/domain/
git commit -m "feat(domain): add GPUWorkload category scoped by GPUNode"
```

---

### Task 2: Model — GPUWorkload struct

**Files:**
- Create: `pkg/models/gpu_workload.go`
- Test: `pkg/models/gpu_workload_test.go`

**Interfaces:**
- Produces: `models.GPUWorkload{ Name, Node, TenantID, Namespace, Model, Runtime string; GPUs int; Mode string; Owner *Tenant }`; methods `GetName() string`, `FilterableFields() []string`, `IsFaulty() bool`, `TenancyOCID(realm string) string`, `TenantName() string`.

- [ ] **Step 1: Write failing test**

Create `pkg/models/gpu_workload_test.go`:

```go
package models

import "testing"

func TestGPUWorkload(t *testing.T) {
	t.Parallel()
	w := GPUWorkload{
		Name: "pod-1", Node: "10.0.0.1", TenantID: "suffix1",
		Namespace: "ns1", Model: "gpt", Runtime: "vllm", GPUs: 2, Mode: "RawDeployment",
	}
	if w.GetName() != "pod-1" {
		t.Errorf("GetName = %q", w.GetName())
	}
	if w.IsFaulty() {
		t.Error("IsFaulty should be false")
	}
	if got := w.TenancyOCID("oc1"); got != "ocid1.tenancy.oc1..suffix1" {
		t.Errorf("TenancyOCID = %q", got)
	}
	// Unresolved owner → raw suffix.
	if got := w.TenantName(); got != "suffix1" {
		t.Errorf("TenantName (unresolved) = %q", got)
	}
	// Resolved owner → tenant name.
	w.Owner = &Tenant{Name: "acme"}
	if got := w.TenantName(); got != "acme" {
		t.Errorf("TenantName (resolved) = %q", got)
	}
	// FilterableFields includes identity fields.
	fields := w.FilterableFields()
	want := []string{"pod-1", "10.0.0.1", "suffix1", "ns1", "gpt", "vllm", "RawDeployment"}
	for _, exp := range want {
		found := false
		for _, f := range fields {
			if f == exp {
				found = true
			}
		}
		if !found {
			t.Errorf("FilterableFields missing %q; got %v", exp, fields)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/models/ -run TestGPUWorkload`
Expected: FAIL — `undefined: GPUWorkload`.

- [ ] **Step 3: Create the model**

Create `pkg/models/gpu_workload.go`:

```go
package models

import "fmt"

// GPUWorkload is a GPU-consuming Kubernetes pod (any pod that requests
// nvidia.com/gpu). Grouped by Node (spec.nodeName, which equals
// GPUNode.Name); its parent category is GPUNode. model/runtime/mode are
// best-effort: blank for non-serving GPU pods.
type GPUWorkload struct {
	Name      string  `json:"name"`
	Node      string  `json:"node"`
	TenantID  string  `json:"tenantId"`
	Namespace string  `json:"namespace"`
	Model     string  `json:"model,omitempty"`
	Runtime   string  `json:"runtime,omitempty"`
	GPUs      int     `json:"gpus"`
	Mode      string  `json:"mode,omitempty"`
	Owner     *Tenant `json:"owner,omitempty"`
}

// GetName returns the pod name.
func (w GPUWorkload) GetName() string { return w.Name }

// IsFaulty is always false; GPUWorkload has no faulty notion.
func (w GPUWorkload) IsFaulty() bool { return false }

// FilterableFields returns the fields matched by `--filter`.
func (w GPUWorkload) FilterableFields() []string {
	return []string{w.Name, w.Node, w.TenantID, w.Namespace, w.Model, w.Runtime, w.Mode}
}

// TenancyOCID returns the full tenancy OCID from realm + tenancy-id suffix.
func (w GPUWorkload) TenancyOCID(realm string) string {
	return fmt.Sprintf("ocid1.tenancy.%s..%s", realm, w.TenantID)
}

// TenantName returns the resolved owning-tenant name, or the raw
// tenancy-id suffix when the owner is unresolved.
func (w GPUWorkload) TenantName() string {
	if w.Owner != nil {
		return w.Owner.Name
	}
	return w.TenantID
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/models/ -run TestGPUWorkload`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/models/gpu_workload.go pkg/models/gpu_workload_test.go
git commit -m "feat(models): add GPUWorkload type"
```

---

### Task 3: Dataset — GPUWorkloadMap + SetGPUWorkloadMap + reset

**Files:**
- Modify: `pkg/models/dataset.go`
- Test: `pkg/models/dataset_test.go`

**Interfaces:**
- Consumes: `models.GPUWorkload` (Task 2); existing `resolveTenantOwnedMap`/`buildTenantIDSuffixMap`.
- Produces: `Dataset.GPUWorkloadMap map[string][]GPUWorkload`; `(*Dataset).SetGPUWorkloadMap(map[string][]GPUWorkload)` — resolves `Owner` per item WITHOUT re-keying (key stays the node).

- [ ] **Step 1: Write failing test**

Add to `pkg/models/dataset_test.go`:

```go
func TestSetGPUWorkloadMap_ResolvesOwnerKeepsNodeKey(t *testing.T) {
	t.Parallel()
	d := &Dataset{
		Tenants: []Tenant{{Name: "acme", IDs: []string{"ocid1.tenancy.oc1..suffix1"}}},
	}
	d.SetGPUWorkloadMap(map[string][]GPUWorkload{
		"node-a": {{Name: "p1", Node: "node-a", TenantID: "suffix1"}},
		"node-b": {{Name: "p2", Node: "node-b", TenantID: "unknown"}},
	})
	// Keyed by node, not re-keyed by tenant.
	if _, ok := d.GPUWorkloadMap["node-a"]; !ok {
		t.Fatalf("expected key node-a; got %v", d.GPUWorkloadMap)
	}
	// Owner resolved for matching suffix.
	if d.GPUWorkloadMap["node-a"][0].Owner == nil || d.GPUWorkloadMap["node-a"][0].Owner.Name != "acme" {
		t.Errorf("owner not resolved: %+v", d.GPUWorkloadMap["node-a"][0].Owner)
	}
	// Unmatched suffix → nil owner, key preserved.
	if d.GPUWorkloadMap["node-b"][0].Owner != nil {
		t.Errorf("expected nil owner for unknown suffix")
	}
}
```

(`buildTenantIDSuffixMap` maps the OCID suffix → tenant index; suffix of `ocid1.tenancy.oc1..suffix1` is `suffix1` — confirm against the existing implementation when writing.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/models/ -run TestSetGPUWorkloadMap`
Expected: FAIL — `GPUWorkloadMap`/`SetGPUWorkloadMap` undefined.

- [ ] **Step 3: Add field, setter, reset**

In `pkg/models/dataset.go`, add the field to the `Dataset` struct (near `GPUNodeMap`):

```go
	GPUNodeMap                        map[string][]GPUNode
	GPUWorkloadMap                    map[string][]GPUWorkload
	DedicatedAIClusterMap             map[string][]DedicatedAICluster
```

Add a per-item resolver (does NOT re-key) and the setter, after `SetImportedModelMap`:

```go
// resolveOwnersInPlace resolves each value's owning Tenant via the
// tenant-suffix map, preserving the original map key (unlike
// resolveTenantOwnedMap, which re-keys by tenant name). Used by
// categories grouped by something other than tenant (e.g. GPUWorkload,
// keyed by node).
func resolveOwnersInPlace[T any](d *Dataset, raw map[string][]T, setOwner func(*T, *Tenant)) map[string][]T {
	suffixMap := d.buildTenantIDSuffixMap()
	for k, v := range raw {
		for i := range v {
			var tenant *Tenant
			if idx, ok := suffixMap[tenantSuffixOf(v[i])]; ok {
				tenant = &d.Tenants[idx]
			}
			setOwner(&v[i], tenant)
		}
		raw[k] = v
	}
	return raw
}
```

Because the generic resolver needs each item's suffix, give it via a tiny closure instead of a `tenantSuffixOf` helper. Replace the body above with this concrete, non-generic setter (simpler and avoids a new interface):

```go
// SetGPUWorkloadMap stores the workload map (keyed by node) and resolves
// each item's owning Tenant from its tenancy-id suffix. The node key is
// preserved (workloads are grouped by node, not tenant).
func (d *Dataset) SetGPUWorkloadMap(m map[string][]GPUWorkload) {
	suffixMap := d.buildTenantIDSuffixMap()
	for k, v := range m {
		for i := range v {
			if idx, ok := suffixMap[v[i].TenantID]; ok {
				v[i].Owner = &d.Tenants[idx]
			} else {
				v[i].Owner = nil
			}
		}
		m[k] = v
	}
	d.GPUWorkloadMap = m
}
```

(Delete the generic `resolveOwnersInPlace` sketch — the concrete setter above is the implementation. It mirrors `buildTenantIDSuffixMap` usage in `resolveTenantOwnedMap`.)

In `ResetRealmScopedFields`, add:

```go
	d.GPUNodeMap = nil
	d.GPUWorkloadMap = nil
	d.DedicatedAIClusterMap = nil
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/models/ -run TestSetGPUWorkloadMap`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/models/dataset.go pkg/models/dataset_test.go
git commit -m "feat(models): GPUWorkloadMap with in-place owner resolution"
```

---

### Task 4: K8s loader — LoadGPUWorkloadsByNode

**Files:**
- Create: `internal/infra/k8s/gpu_workload.go`
- Test: `internal/infra/k8s/gpu_workload_test.go`

**Interfaces:**
- Consumes: `models.GPUWorkload` (Task 2); `gpuProperty` const (`internal/infra/k8s/gpu_node.go`).
- Produces: `k8s.LoadGPUWorkloadsByNode(ctx context.Context, clientset kubernetes.Interface) (map[string][]models.GPUWorkload, error)`.

- [ ] **Step 1: Write failing test**

Create `internal/infra/k8s/gpu_workload_test.go`:

```go
package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/fake"
)

func gpuPod(name, node string, gpus int64, labels, annos map[string]string) *corev1.Pod {
	c := corev1.Container{Name: "main"}
	if gpus > 0 {
		c.Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{gpuProperty: *resource.NewQuantity(gpus, resource.DecimalSI)},
		}
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns1", Labels: labels, Annotations: annos},
		Spec:       corev1.PodSpec{NodeName: node, Containers: []corev1.Container{c}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func TestLoadGPUWorkloadsByNode(t *testing.T) {
	t.Parallel()
	serving := gpuPod("serv", "node-a", 2,
		map[string]string{"tenancy-id": "suffix1", "base-model-name": "gpt", "serving-runtime": "vllm"},
		map[string]string{"ome.io/deploymentMode": "RawDeployment"})
	bare := gpuPod("bare", "node-a", 1, nil, nil)       // GPU pod, no serving labels
	noGPU := gpuPod("nogpu", "node-a", 0, nil, nil)     // excluded
	noNode := gpuPod("nonode", "", 4, nil, nil)         // excluded (unscheduled)

	cs := fake.NewSimpleClientset(serving, bare, noGPU, noNode)
	got, err := LoadGPUWorkloadsByNode(context.Background(), cs)
	if err != nil {
		t.Fatal(err)
	}
	if len(got["node-a"]) != 2 {
		t.Fatalf("want 2 workloads on node-a, got %d (%v)", len(got["node-a"]), got)
	}
	var sv *struct{ found bool }
	_ = sv
	for _, w := range got["node-a"] {
		if w.Name == "serv" {
			if w.Model != "gpt" || w.Runtime != "vllm" || w.GPUs != 2 ||
				w.Mode != "RawDeployment" || w.TenantID != "suffix1" || w.Namespace != "ns1" {
				t.Errorf("serv extraction wrong: %+v", w)
			}
		}
		if w.Name == "bare" && (w.Model != "" || w.Runtime != "" || w.GPUs != 1) {
			t.Errorf("bare extraction wrong: %+v", w)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/infra/k8s/ -run TestLoadGPUWorkloadsByNode`
Expected: FAIL — `undefined: LoadGPUWorkloadsByNode`.

- [ ] **Step 3: Implement the loader**

Create `internal/infra/k8s/gpu_workload.go`:

```go
package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	models "github.com/jingle2008/toolkit/pkg/models"
)

// podGPULimits sums nvidia.com/gpu across the pod's containers' limits.
func podGPULimits(pod *corev1.Pod) int {
	var total int64
	for _, c := range pod.Spec.Containers {
		if q, ok := c.Resources.Limits[gpuProperty]; ok {
			total += q.Value()
		}
	}
	return int(total)
}

// LoadGPUWorkloadsByNode lists running pods that consume GPU and groups
// them by spec.nodeName (== GPUNode.Name). A pod qualifies when it limits
// nvidia.com/gpu > 0 and is scheduled to a node.
func LoadGPUWorkloadsByNode(ctx context.Context, clientset kubernetes.Interface) (map[string][]models.GPUWorkload, error) {
	pods, err := clientset.CoreV1().Pods("").List(ctx, v1.ListOptions{
		FieldSelector: runningPodSelector,
	})
	if err != nil {
		return nil, err
	}

	result := make(map[string][]models.GPUWorkload)
	for i := range pods.Items {
		pod := &pods.Items[i]
		gpus := podGPULimits(pod)
		if gpus <= 0 || pod.Spec.NodeName == "" {
			continue
		}
		labels := pod.Labels
		annos := pod.Annotations
		w := models.GPUWorkload{
			Name:      pod.Name,
			Node:      pod.Spec.NodeName,
			TenantID:  labels["tenancy-id"],
			Namespace: pod.Namespace,
			Model:     labels["base-model-name"],
			Runtime:   labels["serving-runtime"],
			GPUs:      gpus,
			Mode:      annos["ome.io/deploymentMode"],
		}
		result[pod.Spec.NodeName] = append(result[pod.Spec.NodeName], w)
	}
	return result, nil
}
```

(`runningPodSelector = "status.phase=Running"` already exists in `pod_query.go`. The fake clientset ignores field selectors, so the in-loop `phase`-independent filter is fine; real clusters honor it.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/infra/k8s/ -run TestLoadGPUWorkloadsByNode`
Expected: PASS. (Remove the unused `sv` scaffold line from the test if the linter flags it.)

- [ ] **Step 5: Commit**

```bash
git add internal/infra/k8s/gpu_workload.go internal/infra/k8s/gpu_workload_test.go
git commit -m "feat(k8s): LoadGPUWorkloadsByNode from GPU pods"
```

---

### Task 5: Loader interface + production + test doubles

**Files:**
- Modify: `internal/infra/loader/interfaces.go`
- Modify: `internal/infra/loader/production/production.go`
- Modify: any fake/mock loader implementing `loader.Composite` (find via grep)

**Interfaces:**
- Consumes: `k8s.LoadGPUWorkloadsByNode` (Task 4).
- Produces: `loader.GPUWorkloadLoader` interface (in `Composite`); `production.Client.LoadGPUWorkloadsByNode(ctx, kubeCfg, env) (map[string][]models.GPUWorkload, error)`.

- [ ] **Step 1: Add the interface**

In `internal/infra/loader/interfaces.go`, add after `GPUNodeLoader`:

```go
// GPUWorkloadLoader loads GPU-consuming pods grouped by node.
type GPUWorkloadLoader interface {
	LoadGPUWorkloadsByNode(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.GPUWorkload, error)
}
```

Add it to the `Composite` interface list:

```go
	GPUNodeLoader
	GPUWorkloadLoader
	DedicatedAIClusterLoader
```

- [ ] **Step 2: Implement in production**

In `internal/infra/loader/production/production.go`, add (mirrors `LoadGPUNodesByPool`):

```go
// LoadGPUWorkloadsByNode lists GPU-consuming pods grouped by node.
func (Client) LoadGPUWorkloadsByNode(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.GPUWorkload, error) {
	client, err := k8s.NewClientsetFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.LoadGPUWorkloadsByNode(ctx, client)
}
```

- [ ] **Step 3: Update test doubles**

Run: `grep -rln "LoadGPUNodesByPool" --include="*.go" internal/ | grep -i "test\|mock\|fake\|stub"`
For each fake loader that implements `Composite`, add a `LoadGPUWorkloadsByNode` method returning an empty map (or a fixture). Example shape:

```go
func (f *fakeLoader) LoadGPUWorkloadsByNode(ctx context.Context, kubeCfg string, env models.Environment) (map[string][]models.GPUWorkload, error) {
	return f.gpuWorkloads, nil
}
```

- [ ] **Step 4: Build to verify Composite is satisfied**

Run: `go build ./...`
Expected: builds (any type that must satisfy `Composite` now compiles).

- [ ] **Step 5: Commit**

```bash
git add internal/infra/loader/
git commit -m "feat(loader): GPUWorkloadLoader interface + production impl"
```

---

### Task 6: Columns — GPUWorkloadColumns + registry

**Files:**
- Create: `internal/columns/gpu_workload.go`
- Modify: `internal/columns/registry.go`
- Test: `internal/columns/gpu_workload_test.go`

**Interfaces:**
- Consumes: `models.GPUWorkload` (Task 2); `GroupedSet`/`GroupedColumn` types.
- Produces: `columns.GPUWorkloadColumns` (`GroupedSet[models.GPUWorkload]`); registry entry for `domain.GPUWorkload`.

- [ ] **Step 1: Write failing test**

Create `internal/columns/gpu_workload_test.go`:

```go
package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestGPUWorkloadColumns(t *testing.T) {
	t.Parallel()
	// Node MUST be at index 1 (grouped key invariant).
	if GPUWorkloadColumns.Columns[1].Title != "Node" {
		t.Fatalf("col[1] = %q, want Node", GPUWorkloadColumns.Columns[1].Title)
	}
	w := models.GPUWorkload{
		Name: "p1", Node: "node-a", TenantID: "suffix1", Namespace: "ns1",
		Model: "gpt", Runtime: "vllm", GPUs: 2, Mode: "RawDeployment",
		Owner: &models.Tenant{Name: "acme"},
	}
	got := map[string]string{}
	for _, c := range GPUWorkloadColumns.Columns {
		got[c.Key] = c.Render("node-a", w)
	}
	want := map[string]string{
		"name": "p1", "node": "node-a", "tenant": "acme", "namespace": "ns1",
		"model": "gpt", "runtime": "vllm", "gpus": "2", "mode": "RawDeployment",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s = %q, want %q", k, got[k], v)
		}
	}
	if s := GPUWorkloadColumns.RatioSum(); s < 0.98 || s > 1.02 {
		t.Errorf("ratio sum %.3f", s)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/columns/ -run TestGPUWorkloadColumns`
Expected: FAIL — `undefined: GPUWorkloadColumns`.

- [ ] **Step 3: Create the column set**

Create `internal/columns/gpu_workload.go`:

```go
package columns

import (
	"strconv"

	"github.com/jingle2008/toolkit/pkg/models"
)

// GPUWorkloadColumns is the canonical column set for domain.GPUWorkload.
// 8 columns, ratios sum to 1.00. Node is the group key and MUST stay at
// index 1: itemKeyFrom/parentScope derive the scoped key and parent
// (GPUNode) from row[1] for grouped categories.
var GPUWorkloadColumns = GroupedSet[models.GPUWorkload]{Columns: []GroupedColumn[models.GPUWorkload]{
	{
		Title: "Name", Key: "name", Ratio: 0.20, TruncateMiddle: true,
		Render: func(_ string, w models.GPUWorkload) string { return w.Name },
	},
	{
		Title: "Node", Key: "node", Ratio: 0.12,
		Render: func(k string, _ models.GPUWorkload) string { return k },
	},
	{
		Title: "Tenant", Key: "tenant", Ratio: 0.14, TruncateMiddle: true,
		Render: func(_ string, w models.GPUWorkload) string { return w.TenantName() },
		RenderForExport: func(realm, _ string, _ string, w models.GPUWorkload) string {
			return w.TenancyOCID(realm)
		},
	},
	{
		Title: "Namespace", Key: "namespace", Ratio: 0.14, TruncateMiddle: true,
		Render: func(_ string, w models.GPUWorkload) string { return w.Namespace },
	},
	{
		Title: "Model", Key: "model", Ratio: 0.13,
		Render: func(_ string, w models.GPUWorkload) string { return w.Model },
	},
	{
		Title: "Runtime", Key: "runtime", Ratio: 0.13, TruncateMiddle: true,
		Render: func(_ string, w models.GPUWorkload) string { return w.Runtime },
	},
	{
		Title: "GPUs", Key: "gpus", Ratio: 0.06,
		Render: func(_ string, w models.GPUWorkload) string { return strconv.Itoa(w.GPUs) },
	},
	{
		Title: "Mode", Key: "mode", Ratio: 0.08,
		Render: func(_ string, w models.GPUWorkload) string { return w.Mode },
	},
}}
```

(Confirm the `RenderForExport` signature against `ImportedModelColumns` — `func(realm, region string, key string, item T) string` — and match it exactly.)

- [ ] **Step 4: Register the column set**

In `internal/columns/registry.go`, add to the `registry` map:

```go
	domain.GPUNode:                         newGroupedEntry(GPUNodeColumns),
	domain.GPUWorkload:                     newGroupedEntry(GPUWorkloadColumns),
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/columns/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/columns/gpu_workload.go internal/columns/gpu_workload_test.go internal/columns/registry.go
git commit -m "feat(columns): GPUWorkload column set + registry"
```

---

### Task 7: TUI row source + parentScope/itemKeyFrom + invariant test

**Files:**
- Modify: `internal/ui/tui/row_sources.go`
- Modify: `internal/ui/tui/table_utils.go` (`itemKeyFrom`, `parentScope`)
- Test: `internal/ui/tui/row_invariant_test.go`, `internal/ui/tui/parent_nav_test.go`

**Interfaces:**
- Consumes: `columns.GPUWorkloadColumns` (Task 6); `Dataset.GPUWorkloadMap` (Task 3); `domain.GPUWorkload`/`domain.GPUNode`.
- Produces: `rowSources[domain.GPUWorkload]`; `itemKeyFrom`/`parentScope` handle `GPUWorkload`/`GPUNode` parent.

- [ ] **Step 1: Write failing tests**

In `internal/ui/tui/parent_nav_test.go`, add a `TestParentScope` case:

```go
		{"gpu workload", domain.GPUWorkload, table.Row{"pod1", "node-a"}, domain.Scope{Category: domain.GPUNode, Name: "node-a"}, true},
```

In `internal/ui/tui/row_invariant_test.go` `TestGroupKeyAtRowIndex1`, add a case to the `cases` slice:

```go
		{
			name:     "GPUWorkload",
			category: domain.GPUWorkload,
			items: map[string][]models.GPUWorkload{
				tenant: {{Name: name, Node: tenant, Namespace: other}},
			},
		},
```

(Here the group key is the node; the test's `tenant` sentinel doubles as the node value, and the assertion checks the parent scope name equals it. Adjust the assertion block if it hard-codes `domain.Tenant` — for GPUWorkload the parent category is `domain.GPUNode`. If the shared assertion can't express both, add a dedicated `TestParentScope` case only and skip the invariant-loop entry.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/tui/ -run 'TestParentScope|TestGroupKeyAtRowIndex1'`
Expected: FAIL — `parentScope` returns `false` for GPUWorkload (GPUNode parent not handled) and/or no row source.

- [ ] **Step 3: Handle GPUNode parent in parentScope**

In `internal/ui/tui/table_utils.go` `parentScope`, add `domain.GPUNode` to the grouped (row[1]) case:

```go
	case domain.Tenant, domain.GPUPool, domain.GPUNode:
		if len(row) < 2 {
			return domain.Scope{}, false
		}
		return domain.Scope{Category: parent, Name: row[1]}, true
```

- [ ] **Step 4: Handle GPUWorkload in itemKeyFrom**

In `itemKeyFrom`, add `domain.GPUWorkload` to the grouped `ScopedItemKey` case:

```go
	case domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride,
		domain.PropertyTenancyOverride, domain.GPUNode, domain.DedicatedAICluster,
		domain.ImportedModel, domain.ModelArtifact, domain.GPUWorkload:
		return models.ScopedItemKey{Scope: row[1], Name: row[0]}
```

- [ ] **Step 5: Register the row source**

In `internal/ui/tui/row_sources.go`, add (after the GPUNode entry):

```go
	domain.GPUWorkload: groupedSource(columns.GPUWorkloadColumns, domain.GPUNode,
		func(d *models.Dataset) map[string][]models.GPUWorkload { return d.GPUWorkloadMap }),
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/ui/tui/ -run 'TestParentScope|TestGroupKeyAtRowIndex1'`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/tui/row_sources.go internal/ui/tui/table_utils.go internal/ui/tui/row_invariant_test.go internal/ui/tui/parent_nav_test.go
git commit -m "feat(tui): GPUWorkload row source + GPUNode parent resolution"
```

---

### Task 8: Keys — SortGpus + GpusCol + catContext

**Files:**
- Modify: `internal/ui/tui/common/constants.go`
- Modify: `internal/ui/tui/table_sort.go`
- Modify: `internal/ui/tui/keys/registry.go`
- Test: `internal/ui/tui/keys/registry_test.go` (or the existing keys test)

**Interfaces:**
- Consumes: `domain.GPUWorkload`; existing `Parent`, `SortTenant`, `Refresh` bindings.
- Produces: `common.GpusCol = "GPUs"`; `keys.SortGpus` binding (`G`); `catContext[domain.GPUWorkload]`.

- [ ] **Step 1: Write failing test**

Add to the keys test (mirror an existing `ResolveKeys` test). If `internal/ui/tui/keys` lacks a test file, add to `parent_nav_test.go` in the `tui` package:

```go
func TestGPUWorkloadKeys(t *testing.T) {
	t.Parallel()
	km := keys.ResolveKeys(domain.GPUWorkload, common.ListView)
	wantDescs := map[string]bool{"Parent": false, keys.SortPrefix + common.TenantCol: false, keys.SortPrefix + common.GpusCol: false}
	for _, b := range km.Context {
		if _, ok := wantDescs[b.Help().Desc]; ok {
			wantDescs[b.Help().Desc] = true
		}
	}
	for d, found := range wantDescs {
		if !found {
			t.Errorf("GPUWorkload list view missing binding %q", d)
		}
	}
}
```

(Confirm `keys.SortPrefix` is exported; if not, assert on the literal `"sort by GPUs"`-style desc actually produced.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run TestGPUWorkloadKeys`
Expected: FAIL — `common.GpusCol`/`keys.SortGpus` undefined.

- [ ] **Step 3: Add GpusCol constant**

In `internal/ui/tui/common/constants.go`, add (value must equal the column Title "GPUs"):

```go
	// GpusCol is the column name for "GPUs".
	GpusCol = "GPUs"
```

- [ ] **Step 4: Make GPUs sort numerically**

In `internal/ui/tui/table_sort.go`, add `GpusCol` to `intCols`:

```go
	intCols := map[string]struct{}{
		common.FreeCol:    {},
		common.ContextCol: {},
		common.GpusCol:    {},
	}
```

- [ ] **Step 5: Add SortGpus binding + catContext**

In `internal/ui/tui/keys/registry.go`, add the binding (near `SortSize`):

```go
	// SortGpus is a key binding for sorting by the "GPUs" column.
	SortGpus = key.NewBinding(
		key.WithKeys("G"),
		key.WithHelp("<shift+g>", SortPrefix+common.GpusCol),
	)
```

Add the catContext entry (after the `domain.GPUNode` block):

```go
	domain.GPUWorkload: {
		common.ListView: {Parent, SortTenant, SortGpus, Refresh},
	},
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/ui/tui/ -run TestGPUWorkloadKeys`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/tui/common/constants.go internal/ui/tui/table_sort.go internal/ui/tui/keys/registry.go internal/ui/tui/parent_nav_test.go
git commit -m "feat(tui): GPUWorkload key bindings + numeric GPUs sort"
```

---

### Task 9: TUI load wiring (message, command, route, handler, dispatch)

**Files:**
- Modify: `internal/ui/tui/messages.go`
- Modify: `internal/ui/tui/loader_cmd.go`
- Modify: `internal/ui/tui/route_list_loaded.go`
- Modify: `internal/ui/tui/model_reducer.go`
- Modify: `internal/ui/tui/reducer_category.go`
- Modify: `internal/ui/tui/model_update.go`
- Test: `internal/ui/tui/model_reducer_test.go` (or a focused new test)

**Interfaces:**
- Consumes: `loadGPUWorkloadsCmd`, `gpuWorkloadsLoadedMsg`, `(*Dataset).SetGPUWorkloadMap`, `(*Model).applyDataset`.
- Produces: full load chain so selecting `GPUWorkload` triggers a load and populates `GPUWorkloadMap`.

- [ ] **Step 1: Write failing test**

Add to `internal/ui/tui/model_reducer_test.go` (mirror an existing `handle…Loaded` test):

```go
func TestHandleGPUWorkloadsLoaded(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset = &models.Dataset{}
	items := map[string][]models.GPUWorkload{"node-a": {{Name: "p1", Node: "node-a"}}}
	m.handleGPUWorkloadsLoaded(items, m.gen)
	if got := m.dataset.GPUWorkloadMap["node-a"]; len(got) != 1 || got[0].Name != "p1" {
		t.Fatalf("GPUWorkloadMap not applied: %+v", m.dataset.GPUWorkloadMap)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run TestHandleGPUWorkloadsLoaded`
Expected: FAIL — `handleGPUWorkloadsLoaded` undefined.

- [ ] **Step 3: Add the message type**

In `internal/ui/tui/messages.go`, after `gpuNodesLoadedMsg`:

```go
type gpuWorkloadsLoadedMsg struct {
	Items map[string][]models.GPUWorkload
	Gen   int
}
```

- [ ] **Step 4: Add the load command**

In `internal/ui/tui/loader_cmd.go`, after `loadGPUNodesCmd`:

```go
func loadGPUWorkloadsCmd(ctx context.Context, ld loader.Composite, kubeCfg string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadGPUWorkloadsByNode(ctx, kubeCfg, env)
		if err != nil {
			return errMsg(fmt.Errorf("failed to load %s: %w", domain.GPUWorkload, err))
		}
		return gpuWorkloadsLoadedMsg{Items: items, Gen: gen}
	}
}
```

- [ ] **Step 5: Add the handler**

In `internal/ui/tui/model_reducer.go`, after `handleGPUNodesLoaded`:

```go
func (m *Model) handleGPUWorkloadsLoaded(items map[string][]models.GPUWorkload, gen int) {
	if gen != m.gen {
		m.endTask(true)
		return
	}
	total := 0
	for _, v := range items {
		total += len(v)
	}
	m.applyDataset(func(ds *models.Dataset) { ds.SetGPUWorkloadMap(items) }, domain.GPUWorkload, total)
}
```

- [ ] **Step 6: Route the message**

In `internal/ui/tui/route_list_loaded.go`, add a case:

```go
	case gpuNodesLoadedMsg:
		m.handleGPUNodesLoaded(msg.Items, msg.Gen)
	case gpuWorkloadsLoadedMsg:
		m.handleGPUWorkloadsLoaded(msg.Items, msg.Gen)
```

In `internal/ui/tui/model_update.go`, add `gpuWorkloadsLoadedMsg` to the batched case list:

```go
	case baseModelsLoadedMsg, importedModelsLoadedMsg, gpuPoolsLoadedMsg,
		gpuNodesLoadedMsg, gpuWorkloadsLoadedMsg, dedicatedAIClustersLoadedMsg, tenancyOverridesLoadedMsg,
		limitRegionalOverridesLoadedMsg, consolePropertyRegionalOverridesLoadedMsg,
		propertyRegionalOverridesLoadedMsg:
```

- [ ] **Step 7: Add the category dispatch + load guard**

In `internal/ui/tui/reducer_category.go`, add the handler (after `handleGPUNodeCategory`):

```go
func (m *Model) handleGPUWorkloadCategory(refresh bool, gen int) tea.Cmd {
	if m.dataset == nil || m.dataset.GPUWorkloadMap == nil || refresh {
		return loadGPUWorkloadsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	}
	return nil
}
```

Add to the `handlers` dispatch map:

```go
		domain.GPUNode:                         func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleGPUNodeCategory(refresh, gen) },
		domain.GPUWorkload:                     func(m *Model, refresh bool, gen int) tea.Cmd { return m.handleGPUWorkloadCategory(refresh, gen) },
```

- [ ] **Step 8: Run test + build**

Run: `go test ./internal/ui/tui/ -run TestHandleGPUWorkloadsLoaded && go build ./...`
Expected: PASS + build.

- [ ] **Step 9: Commit**

```bash
git add internal/ui/tui/
git commit -m "feat(tui): wire GPUWorkload load (msg/cmd/route/handler/dispatch)"
```

---

### Task 10: CLI get parity

**Files:**
- Modify: `internal/cli/get.go`
- Test: `internal/cli/get_test.go` (if a per-category test pattern exists) — otherwise rely on the snapshot/build.

**Interfaces:**
- Consumes: `loader.Composite.LoadGPUWorkloadsByNode`.
- Produces: `toolkit get gw` renders the workload map.

- [ ] **Step 1: Add the case**

In `internal/cli/get.go`, add a case alongside `domain.GPUNode` (mirror it):

```go
	case domain.GPUWorkload:
		grouped, err := ld.LoadGPUWorkloadsByNode(ctx, cfg.KubeConfig, env)
		if err != nil {
			return fmt.Errorf("load gpu workloads: %w", err)
		}
		return writeMap(w, collections.FilterMapOrAll(grouped, filter), limit, opts, domain.GPUWorkload, env, selected)
```

- [ ] **Step 2: Build + smoke test**

Run: `go build ./... && go test ./internal/cli/`
Expected: builds; CLI tests pass. (Owner is unresolved in the CLI path, so the Tenant column shows the raw suffix — consistent with DAC/ImportedModel CLI output.)

- [ ] **Step 3: Commit**

```bash
git add internal/cli/get.go
git commit -m "feat(cli): get GPUWorkload (gw)"
```

---

### Task 11: Full verification

- [ ] **Step 1: Build, vet, test**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all pass.

- [ ] **Step 2: Manual TUI sanity (optional)**

Launch the TUI, navigate to `GPUWorkload` (alias `gw`), drill in from a GPU node (Enter), jump back (`o`), sort by GPUs (`shift+g`) and Tenant (`shift+t`).

- [ ] **Step 3: Final commit / open PR**

```bash
git push -u origin feat/gpu-workload-category
```

---

## Self-Review

- **Spec coverage:** domain (T1), model (T2), dataset+resolution (T3), loader (T4), interface/production/doubles (T5), columns+registry (T6), row source + parentScope/itemKeyFrom + invariant (T7), keys + GPUs sort (T8), load wiring (T9), CLI parity (T10), verification (T11). MCP parity from the spec is intentionally deferred — note below.
- **Deviation from spec:** MCP `get`-equivalent is NOT included (the spec listed CLI/MCP parity). MCP adds a tool registration surface that isn't needed for the TUI/CLI goal; deferred to a follow-up. Flag if MCP parity is required for v1.
- **Type consistency:** `LoadGPUWorkloadsByNode`, `GPUWorkloadMap`, `SetGPUWorkloadMap`, `gpuWorkloadsLoadedMsg`, `loadGPUWorkloadsCmd`, `handleGPUWorkloadsLoaded`, `handleGPUWorkloadCategory`, `GPUWorkloadColumns`, `GpusCol`, `SortGpus` used consistently across tasks.
- **Verify-when-writing:** `RenderForExport` exact signature (T6), `buildTenantIDSuffixMap` suffix semantics (T3), `keys.SortPrefix` export (T8), and the shared invariant-test assertion shape (T7) are called out to confirm against the real code at implementation time.
```
