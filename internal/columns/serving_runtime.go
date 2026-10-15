package columns

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

/*
servingRuntimeHardware renders the merged hardware constraint.

Accelerator classes and instance types are both optional and express
the same intent at different granularity, so they share a column.
Accelerator classes win when present — they encode tensor-parallel
sizing, so they're the more precise statement of what the runtime
needs. The ac:/it: marker keeps which constraint is in force visible;
without it the two are indistinguishable once collapsed, and they fail
scheduling in different ways.
*/
func servingRuntimeHardware(r models.ServingRuntime) string {
	if len(r.AcceleratorClasses) > 0 {
		return fmt.Sprintf("ac: %s (%d)", strings.Join(r.GPUFamilies(), ", "), len(r.AcceleratorClasses))
	}
	if len(r.InstanceTypes) > 0 {
		return "it: " + strings.Join(r.InstanceTypes, ", ")
	}
	return ""
}

/*
servingRuntimeImage drops the registry host and org from an image ref,
leaving name:tag.

Those leading segments are near-constant across a fleet, so they cost
about half the cell to distinguish nothing, while the name and tag are
what identify the runtime build. The tradeoff: two same-named images
from different registries render identically. The full ref stays in
`-o json`.
*/
func servingRuntimeImage(r models.ServingRuntime) string {
	if r.Image == "" {
		return ""
	}
	if idx := strings.LastIndex(r.Image, "/"); idx >= 0 {
		return r.Image[idx+1:]
	}
	return r.Image
}

// servingRuntimeSizeRange renders spec.modelSizeRange as a range, or
// just the bound that exists when only one is set.
func servingRuntimeSizeRange(r models.ServingRuntime) string {
	switch {
	case r.ModelSizeMin != "" && r.ModelSizeMax != "":
		return r.ModelSizeMin + "–" + r.ModelSizeMax
	case r.ModelSizeMin != "":
		return r.ModelSizeMin + "–"
	case r.ModelSizeMax != "":
		return "–" + r.ModelSizeMax
	}
	return ""
}

/*
servingRuntimeCPUMem pairs the two quantities that are almost always
read together, freeing a column for the GPU count.

Limits win, with requests as the fallback: many runtimes cap only the
GPU and express cpu/memory as requests alone, so limits-only would
leave the cell blank for a sizeable share of a real fleet. A `req:`
marker flags a fallback value, because a scheduling floor and a cap
are different promises and a bare number would imply the stricter one.
The marker appears if either half fell back — mixed rows are reported
as requests, the weaker claim.
*/
func servingRuntimeCPUMem(r models.ServingRuntime) string {
	cpu, cpuFromReq := orRequest(r.CPULimit, r.CPURequest)
	mem, memFromReq := orRequest(r.MemoryLimit, r.MemoryRequest)
	if cpu == "" && mem == "" {
		return ""
	}
	if cpuFromReq || memFromReq {
		return "req: " + cpu + " / " + mem
	}
	return cpu + " / " + mem
}

// servingRuntimeGPU falls back the same way, unmarked: the column is
// too narrow for a marker, and in practice the GPU is the one resource
// runtimes almost always cap explicitly.
func servingRuntimeGPU(r models.ServingRuntime) string {
	gpu, _ := orRequest(r.GPULimit, r.GPURequest)
	return gpu
}

// orRequest returns the limit when set, otherwise the request, and
// reports whether it fell back.
func orRequest(limit, request string) (string, bool) {
	if limit != "" {
		return limit, false
	}
	return request, request != ""
}

// ServingRuntimeColumns is the canonical column set for
// domain.ServingRuntime. 8 columns, ratios sum to 1.00. Name and
// Hardware lead because they're what you scan for; the two flags sit
// last and narrow, since a disabled row is already styled as faulty
// across its whole width.
var ServingRuntimeColumns = Set[models.ServingRuntime]{Columns: []Column[models.ServingRuntime]{
	{
		Title: "Name", Key: "name", Ratio: 0.20,
		Render: func(r models.ServingRuntime) string { return r.Name },
	},
	{
		Title: "Hardware", Key: "hardware", Ratio: 0.26, TruncateMiddle: true,
		Render: servingRuntimeHardware,
	},
	{
		Title: "Image", Key: "image", Ratio: 0.20, TruncateMiddle: true,
		Render: servingRuntimeImage,
	},
	{
		Title: "Size Range", Key: "size-range", Ratio: 0.10,
		Render: servingRuntimeSizeRange,
	},
	{
		Title: "CPU/Mem", Key: "cpu-mem", Ratio: 0.10,
		Render: servingRuntimeCPUMem,
	},
	{
		// "GPUs" rather than "GPU" so the single SortGPUs binding covers
		// this column as well as GPUPool's and GPUWorkload's — sort
		// bindings match on column title, so a divergent title here
		// would leave the key silently doing nothing.
		Title: "GPUs", Key: "gpus", Ratio: 0.05,
		Render: servingRuntimeGPU,
	},
	{
		Title: "Auto-Select", Key: "auto-select", Ratio: 0.05,
		Render: func(r models.ServingRuntime) string { return fmt.Sprint(r.AutoSelect) },
	},
	{
		Title: "Disabled", Key: "disabled", Ratio: 0.04,
		Render: func(r models.ServingRuntime) string { return fmt.Sprint(r.Disabled) },
	},
}}

/*
servingRuntimeGPUPerNode renders the GPU count as per-node × nodes for
multi-node runtimes ("8×2"), and as the plain count otherwise.

The stored value is a total across all pods, which for a multi-node
runtime reads as one impossibly large node; placement needs the
per-node figure. Totals that don't divide evenly (heterogeneous leader
and workers) or don't parse stay as the raw total rather than
misreporting either half.
*/
func servingRuntimeGPUPerNode(r models.ServingRuntime) string {
	total := servingRuntimeGPU(r)
	if r.NodeCount <= 1 {
		return total
	}
	n, err := strconv.Atoi(total)
	if err != nil || n%r.NodeCount != 0 {
		return total
	}
	return fmt.Sprintf("%d×%d", n/r.NodeCount, r.NodeCount)
}

/*
ServingRuntimeTUIColumns is the TUI's slimmer view of a ServingRuntime,
built for scanning placement (Hardware, GPUs) and rollout (Image), with Status in place of
the CLI's Auto-Select and Disabled columns.

It is a separate Set rather than a filter over ServingRuntimeColumns
because the cells differ too, not just the column list, and the CLI's
`toolkit get` output and --columns keys must not change underneath
scripts. Size range and cpu/memory remain in the detail view's JSON.
5 columns, ratios sum to 1.00, sized so a 120-column terminal shows
typical rows untruncated.
*/
var ServingRuntimeTUIColumns = Set[models.ServingRuntime]{Columns: []Column[models.ServingRuntime]{
	{
		Title: "Name", Key: "name", Ratio: 0.26,
		Render: func(r models.ServingRuntime) string { return r.Name },
	},
	{
		Title: "Hardware", Key: "hardware", Ratio: 0.3, TruncateMiddle: true,
		// GPU families only, normalized across both constraint kinds; the
		// raw lists stay in the detail view.
		Render: func(r models.ServingRuntime) string { return strings.Join(r.GPUFamilies(), ", ") },
	},
	{
		// Title kept as "GPUs" for the shared SortGPUs binding, as above;
		// 0.07 is the least that fits "GPUs ↕" at 120 columns.
		Title: "GPUs", Key: "gpus", Ratio: 0.07,
		Render: servingRuntimeGPUPerNode,
	},
	{
		Title: "Image", Key: "image", Ratio: 0.3, TruncateMiddle: true,
		Render: servingRuntimeImage,
	},
	{
		// Status folds spec.disabled and autoSelect into one word; see
		// models.ServingRuntime.Status.
		Title: "Status", Key: "status", Ratio: 0.07,
		Render: models.ServingRuntime.Status,
	},
}}
