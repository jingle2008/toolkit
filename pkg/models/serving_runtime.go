package models

/*
ServingRuntime represents an OME ClusterServingRuntime CR — the
cluster-scoped template describing how a class of models is served:
which hardware it may land on, which container image runs it, and what
resources that container is capped at.

Fields hold the CR's values verbatim. The table cells that collapse
them (accelerator families, `name:tag` images) are rendered in
internal/columns, so `-o json` stays lossless.
*/
type ServingRuntime struct {
	Name string `json:"name"`

	// AcceleratorClasses is spec.acceleratorRequirements.acceleratorClasses,
	// e.g. "nvidia-a100-80gb-4" — family plus tensor-parallel size.
	AcceleratorClasses []string `json:"acceleratorClasses,omitempty"`
	// InstanceTypes is the node.kubernetes.io/instance-type values from
	// the required node affinity, e.g. "BM.GPU.H100.8". Both this and
	// AcceleratorClasses are optional, and a runtime may set either,
	// both, or neither.
	InstanceTypes []string `json:"instanceTypes,omitempty"`

	// Image is the full ome-container image reference, registry included.
	Image string `json:"image,omitempty"`

	// ModelSizeMin/Max are spec.modelSizeRange, kept as the CR's own
	// strings ("1.8B") rather than parsed — the suffixes are not a
	// closed set and a wrong parse would silently misreport capacity.
	ModelSizeMin string `json:"modelSizeMin,omitempty"`
	ModelSizeMax string `json:"modelSizeMax,omitempty"`

	// CPULimit, MemoryLimit and GPULimit are the ome-container's
	// resources.limits entries, as Kubernetes quantity strings ("500m",
	// "30Gi", "1"). Strings, not numbers: quantities carry units and
	// round-tripping them through a numeric type loses fidelity.
	CPULimit    string `json:"cpuLimit,omitempty"`
	MemoryLimit string `json:"memoryLimit,omitempty"`
	GPULimit    string `json:"gpuLimit,omitempty"`

	// CPURequest, MemoryRequest and GPURequest are the same entries from
	// resources.requests. Kept separate rather than folded into the
	// Limit fields: a request is a scheduling floor and a limit is a
	// cap, and reporting one under the other's name would be wrong.
	// Many runtimes cap only the GPU and express cpu/memory as requests
	// alone, which is why the table falls back to these.
	CPURequest    string `json:"cpuRequest,omitempty"`
	MemoryRequest string `json:"memoryRequest,omitempty"`
	GPURequest    string `json:"gpuRequest,omitempty"`

	// Disabled mirrors spec.disabled.
	Disabled bool `json:"disabled"`
	// AutoSelect is true when any spec.supportedModelFormats entry sets
	// autoSelect — the flag is per-format, but the question it answers
	// ("can the scheduler pick this runtime on its own") is per-runtime.
	AutoSelect bool `json:"autoSelect"`
}

// GetName returns the name of the serving runtime.
func (r ServingRuntime) GetName() string {
	return r.Name
}

/*
FilterableFields returns the fields the fuzzy filter matches against.

The raw accelerator classes and instance types are included even though
the Hardware column renders a collapsed form, so filtering on a
tensor-parallel variant ("h100-8") or on a registry host still finds
the row whose cell shows neither.
*/
func (r ServingRuntime) FilterableFields() []string {
	fields := make([]string, 0, len(r.AcceleratorClasses)+len(r.InstanceTypes)+4)
	fields = append(fields, r.Name, r.Image, r.ModelSizeMin, r.ModelSizeMax)
	fields = append(fields, r.AcceleratorClasses...)
	fields = append(fields, r.InstanceTypes...)
	return fields
}

// IsFaulty reports a disabled runtime as faulty so the TUI's
// faulty-only filter and row styling surface it. A disabled runtime
// serves nothing, which is what an operator scanning for problems
// wants to see.
func (r ServingRuntime) IsFaulty() bool {
	return r.Disabled
}
