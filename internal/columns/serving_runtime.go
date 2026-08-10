package columns

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

// tpSuffix matches the trailing tensor-parallel size on an accelerator
// class ("nvidia-a100-80gb-4" -> "-4"), which is what distinguishes the
// variants of one GPU family from each other.
var tpSuffix = regexp.MustCompile(`-\d+$`)

/*
acceleratorFamilies collapses accelerator classes to their distinct GPU
families, in first-seen order.

"nvidia-a100-80gb-1", "nvidia-a100-80gb-4" and "nvidia-a100-80gb-8" all
describe the same hardware at different tensor-parallel sizes, so a
cell listing all nineteen answers "which GPUs" far worse than a cell
listing five. Values that don't match the nvidia-<family>-<tp> shape
pass through whole rather than being dropped — an unfamiliar naming
scheme should look odd, not vanish.
*/
func acceleratorFamilies(classes []string) []string {
	seen := make(map[string]struct{}, len(classes))
	families := make([]string, 0, len(classes))
	for _, c := range classes {
		f := tpSuffix.ReplaceAllString(strings.TrimPrefix(c, "nvidia-"), "")
		if f == "" {
			continue
		}
		if _, dup := seen[f]; dup {
			continue
		}
		seen[f] = struct{}{}
		families = append(families, f)
	}
	return families
}

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
		families := acceleratorFamilies(r.AcceleratorClasses)
		return fmt.Sprintf("ac: %s (%d)", strings.Join(families, ", "), len(r.AcceleratorClasses))
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

// servingRuntimeCPUMem pairs the two limits that are almost always read
// together, freeing a column for the GPU count.
func servingRuntimeCPUMem(r models.ServingRuntime) string {
	if r.CPULimit == "" && r.MemoryLimit == "" {
		return ""
	}
	return r.CPULimit + " / " + r.MemoryLimit
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
		Title: "GPU", Key: "gpu", Ratio: 0.05,
		Render: func(r models.ServingRuntime) string { return r.GPULimit },
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
