/*
Package terraform provides functions for loading and managing infrastructure data from Terraform state and configuration.
*/
package terraform

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/tryfunc"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"

	"github.com/jingle2008/toolkit/internal/fileutil"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	models "github.com/jingle2008/toolkit/pkg/models"
)

// ErrModelArtifactMapNotResolved is returned when the model artifact map cannot be resolved.
var ErrModelArtifactMapNotResolved = errors.New("model artifact map not resolved")

/*
Constants for local and remote chart locations.
*/
const (
	localKey    = "local"
	localBlock  = "locals"
	varKey      = "var"
	dataKey     = "data"
	pathKey     = "path"
	moduleKey   = "module"
	outputBlock = "output"
	outputValue = "value"
)

var localFuncMap = map[string]function.Function{
	"format":     stdlib.FormatFunc,
	"lookup":     stdlib.LookupFunc,
	"merge":      stdlib.MergeFunc,
	"join":       stdlib.JoinFunc,
	"contains":   stdlib.ContainsFunc,
	"keys":       stdlib.KeysFunc,
	"flatten":    stdlib.FlattenFunc,
	"distinct":   stdlib.DistinctFunc,
	"toset":      stdlib.MakeToFunc(cty.Set(cty.DynamicPseudoType)),
	"tolist":     stdlib.MakeToFunc(cty.List(cty.DynamicPseudoType)),
	"tomap":      stdlib.MakeToFunc(cty.Map(cty.DynamicPseudoType)),
	"file":       fileFunc,
	"concat":     stdlib.ConcatFunc,
	"split":      stdlib.SplitFunc,
	"regex":      stdlib.RegexFunc,
	"try":        tryfunc.TryFunc,
	"can":        tryfunc.CanFunc,
	"startswith": startsWithFunc,
	"alltrue":    allTrueFunc,
}

// startsWithFunc is terraform's startswith(): whether str begins with
// prefix.
var startsWithFunc = function.New(&function.Spec{
	Params: []function.Parameter{{Name: "str", Type: cty.String}, {Name: "prefix", Type: cty.String}},
	Type:   function.StaticReturnType(cty.Bool),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		return cty.BoolVal(strings.HasPrefix(args[0].AsString(), args[1].AsString())), nil
	},
})

// allTrueFunc is terraform's alltrue(): whether every element of a list
// of bools is true (true for an empty list). Like terraform's, it is
// unknown once it reaches an unknown element — cty only short-circuits
// an unknown argument, not a known list holding one.
var allTrueFunc = function.New(&function.Spec{
	Params: []function.Parameter{{Name: "list", Type: cty.List(cty.Bool)}},
	Type:   function.StaticReturnType(cty.Bool),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		for it := args[0].ElementIterator(); it.Next(); {
			_, v := it.Element()
			if !v.IsKnown() {
				return cty.UnknownVal(cty.Bool), nil
			}
			if v.IsNull() || !v.True() {
				return cty.False, nil
			}
		}
		return cty.True, nil
	},
})

// fileFunc is terraform's file(): the UTF-8 contents of the file at path.
// Modules build the path from path.module, so it resolves the same way
// whatever the working directory.
var fileFunc = function.New(&function.Spec{
	Params: []function.Parameter{{Name: "path", Type: cty.String}},
	Type:   function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		path := args[0].AsString()
		b, err := os.ReadFile(path) //nolint:gosec // a path the module names, under the repo the user configured
		if err != nil {
			return cty.NilVal, err
		}
		if !utf8.Valid(b) {
			return cty.NilVal, fmt.Errorf("contents of %s are not valid UTF-8", path)
		}
		return cty.StringVal(string(b)), nil
	},
})

func getLocalAttributesDI(
	ctx context.Context,
	dirPath string,
	listFilesFunc func(context.Context, string, string) ([]string, error),
	updateLocalAttributesFunc func(string, hclsyntax.Attributes) error,
) (hclsyntax.Attributes, error) {
	tfFiles, err := listFilesFunc(ctx, dirPath, ".tf")
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	attributes := make(hclsyntax.Attributes)
	for _, file := range tfFiles {
		if err := updateLocalAttributesFunc(file, attributes); err != nil {
			return nil, fmt.Errorf("update local attributes: %w", err)
		}
	}

	return attributes, nil
}

/*
LoadLocalAttributes loads and returns all local attributes from Terraform files in the specified directory.
*/
func LoadLocalAttributes(ctx context.Context, dirPath string) (hclsyntax.Attributes, error) {
	return getLocalAttributesDI(ctx, dirPath, fileutil.ListFiles, updateLocalAttributes)
}

func updateLocalAttributes(filepath string, attributes hclsyntax.Attributes) error {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCLFile(filepath)
	if diags.HasErrors() {
		return fmt.Errorf("terraform diagnostics error: %w", errors.New(diags.Error()))
	}

	// find `locals` block
	for _, block := range file.Body.(*hclsyntax.Body).Blocks {
		switch block.Type {
		case localBlock:
			maps.Copy(attributes, block.Body.Attributes)
		case outputBlock:
			key := block.Labels[0]
			value := block.Body.Attributes[outputValue]
			attributes[key] = value
		}
	}

	return nil
}

// getVariableDefaults parses `variable "name" { default = ... }` blocks
// in every .tf file under dirPath and returns the evaluated defaults.
// Variables without a default are omitted; references to them will then
// remain unresolved, matching how terraform behaves without -var input.
func getVariableDefaults(ctx context.Context, dirPath string) (map[string]cty.Value, error) {
	logger := logging.FromContext(ctx)
	tfFiles, err := fileutil.ListFiles(ctx, dirPath, ".tf")
	if err != nil {
		return nil, err
	}
	defaults := make(map[string]cty.Value)
	for _, fpath := range tfFiles {
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCLFile(fpath)
		if diags.HasErrors() {
			continue
		}
		body, ok := file.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		for _, block := range body.Blocks {
			if block.Type != "variable" || len(block.Labels) == 0 {
				continue
			}
			defAttr, ok := block.Body.Attributes["default"]
			if !ok {
				continue
			}
			val, vdiags := defAttr.Expr.Value(nil)
			if vdiags.HasErrors() {
				// Defaults that reference var/local/data or use functions
				// cannot be evaluated with a nil context. Treat as
				// "unset" (matches terraform without -var) but record
				// at debug level so users tracing unresolved refs can
				// see the default existed.
				logger.Debugw("skipping non-literal variable default",
					"var", block.Labels[0], "file", fpath, "errors", vdiags.Errs())
				continue
			}
			defaults[block.Labels[0]] = val
		}
	}
	return defaults, nil
}

func mergeObject(object cty.Value, key string, value cty.Value) cty.Value {
	valueMap := object.AsValueMap()
	if valueMap == nil { // AsValueMap returns nil for an empty object
		valueMap = make(map[string]cty.Value, 1)
	}
	valueMap[key] = value
	return cty.ObjectVal(valueMap)
}

func loadLocalValueMap(ctx context.Context, dirPath string, env models.Environment) (map[string]cty.Value, error) {
	logger := logging.FromContext(ctx)
	attributes, err := LoadLocalAttributes(ctx, dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to decode HCL: %w", err)
	}

	executionTarget := cty.ObjectVal(map[string]cty.Value{
		"region": cty.ObjectVal(map[string]cty.Value{
			"realm":              cty.StringVal(env.Realm),
			"public_name":        cty.StringVal(env.Region),
			"public_domain_name": cty.StringVal("example.com"),
		}),
		"tenancy_ocid": cty.StringVal("ocid1.tenancy.oc1..example"),
		"additional_locals": cty.ObjectVal(map[string]cty.Value{
			"environment": cty.StringVal(env.Type),
		}),
	})

	localObject := cty.ObjectVal(map[string]cty.Value{
		"execution_target": executionTarget,
	})

	// Start from any defaults declared in this module's variables.tf so
	// references like `var.worker_nsgs` resolve to their declared defaults
	// instead of failing. Then layer the toolkit-supplied region /
	// environment on top so they always reflect the active env.
	varDefaults, _ := getVariableDefaults(ctx, dirPath)
	varMap := make(map[string]cty.Value, len(varDefaults)+2)
	maps.Copy(varMap, varDefaults)
	varMap["region"] = cty.StringVal(env.Region)
	varMap["environment"] = cty.StringVal(env.Type)
	varObject := cty.ObjectVal(varMap)

	dataObject := cty.ObjectVal(map[string]cty.Value{
		"oci_identity_availability_domains": createAvailabilityDomains(),
		"oci_objectstorage_namespace":       createObjectStorageNamespace(),
	})

	context := hcl.EvalContext{
		Variables: map[string]cty.Value{
			localKey: localObject,
			varKey:   varObject,
			dataKey:  dataObject,
			pathKey:  cty.ObjectVal(map[string]cty.Value{"module": cty.StringVal(dirPath)}),
		},
		Functions: localFuncMap,
	}
	localObject = resolveLocalsAndModules(ctx, dirPath, &context, localObject, attributes, 0)

	for key := range attributes {
		attr, ok := attributes[key]
		if !ok {
			continue
		}

		value, diags := attr.Expr.Value(&context)
		if diags.HasErrors() {
			logger.Errorw(
				"cannot resolve local",
				"local", key,
				"errors", diags.Errs(),
			)
			continue
		}

		// value.GoString() formats the whole cty value — only pay for it
		// when debug logging is actually enabled.
		if logger.DebugEnabled() {
			logger.Debugw(
				"cannot resolve local value",
				"local", key,
				"value", value.GoString(),
			)
		}
	}

	return localObject.AsValueMap(), nil
}

// LoadServiceTenancies loads ServiceTenancy objects from the given repository path.
//
// The shep_targets locals block mixes tenancy definitions with unrelated
// helper locals (region-group lists, rollout maps built from for-expressions,
// etc.), so tenancies are recognized structurally — any resolved object with
// a string `tenancy_name` attribute — rather than by name. Locals that cannot
// be resolved (e.g. ones referencing variables without defaults) are logged
// and skipped instead of failing the whole load.
func LoadServiceTenancies(ctx context.Context, repoPath string) ([]models.ServiceTenancy, error) {
	logger := logging.FromContext(ctx)
	dirPath := filepath.Join(repoPath, "shared_modules/shep_targets")
	attributes, err := LoadLocalAttributes(ctx, dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse attributes: %w", err)
	}

	varDefaults, _ := getVariableDefaults(ctx, dirPath)
	evalCtx := hcl.EvalContext{
		Variables: map[string]cty.Value{
			localKey: cty.EmptyObjectVal,
			varKey:   cty.ObjectVal(varDefaults),
			pathKey:  cty.ObjectVal(map[string]cty.Value{"module": cty.StringVal(dirPath)}),
		},
		Functions: localFuncMap,
	}
	// The oc1 targets come from a child module (module.oc1_inventory).
	localObject := resolveLocalsAndModules(ctx, dirPath, &evalCtx, cty.EmptyObjectVal, attributes, 0)
	for key := range attributes {
		logger.Warnw("skipping unresolved shep_targets local", "local", key)
	}

	tenancyMap := make(map[string]*models.ServiceTenancy)

	for key, value := range localObject.AsValueMap() {
		if !isTenancyValue(value) {
			continue
		}

		realm := strings.Split(key, "_")[0]
		tenancy := getServiceTenancy(value, realm)
		fullName := fmt.Sprintf("%s-%s", tenancy.Realm, tenancy.Name)
		if t, ok := tenancyMap[fullName]; ok {
			t.Regions = append(t.Regions, tenancy.Regions...)
		} else {
			tenancyMap[fullName] = tenancy
		}
	}

	tenancies := make([]models.ServiceTenancy, 0, len(tenancyMap))
	for _, tenancy := range tenancyMap {
		sort.Strings(tenancy.Regions)
		tenancies = append(tenancies, *tenancy)
	}
	// Map order would shuffle the rows (and the environments derived from
	// them) on every load, so `get --limit` picked different rows each run.
	sort.Slice(tenancies, func(i, j int) bool {
		a, b := tenancies[i], tenancies[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Realm < b.Realm
	})

	return tenancies, nil
}

// isTenancyValue reports whether a resolved local looks like a tenancy
// definition: an object/map with a string `tenancy_name` attribute.
func isTenancyValue(value cty.Value) bool {
	t := value.Type()
	if !t.IsObjectType() && !t.IsMapType() {
		return false
	}
	name, ok := value.AsValueMap()["tenancy_name"]
	return ok && name.Type() == cty.String
}

func getServiceTenancy(object cty.Value, realm string) *models.ServiceTenancy {
	result := models.ServiceTenancy{
		Realm: realm,
	}

	for name, value := range object.AsValueMap() {
		switch name {
		case "tenancy_name":
			result.Name = value.AsString()
		case "home_region":
			result.HomeRegion = value.AsString()
		case "regions":
			regions := make([]string, 0, len(value.AsValueSlice()))
			for _, region := range value.AsValueSlice() {
				regions = append(regions, region.AsString())
			}
			result.Regions = regions
		case "environment":
			result.Environment = value.AsString()
		}
	}

	return &result
}

/*
PartialLoadError signals that some, but not all, of a multi-source loader's
inputs failed. The returned slice still contains usable data from sources
that succeeded; callers that care about completeness should detect it via
errors.As and decide how to surface it (the TUI ignores it; the headless
CLI prints a warning to stderr).
*/
type PartialLoadError struct {
	// Source is a human-readable label, e.g. "GPUPools".
	Source string
	// Errs holds one error per failed source, already wrapped with the
	// source identifier (e.g. "env_nodepools_config: …").
	Errs []error
}

// Error implements error.
func (e *PartialLoadError) Error() string {
	return fmt.Sprintf("%s: %d source(s) failed to load: %v",
		e.Source, len(e.Errs), errors.Join(e.Errs...))
}

// Unwrap returns the per-source errors so errors.Is / errors.As walk into
// them. Requires Go 1.20+.
func (e *PartialLoadError) Unwrap() []error { return e.Errs }

/*
LoadGPUPools loads GPUPool objects from the given repository path and environment.

It reads three sources (self-managed instance pools, self-managed cluster
networks, OKE-managed nodepools). Failures in individual sources are
logged to ctx's logger and the function returns the union of pools from
sources that succeeded. Return modes:

  - All sources succeed: (pools, nil).
  - Some sources fail, some succeed: (pools, *PartialLoadError). Callers
    can use errors.As to detect and decide whether to surface the
    partial-failure warning.
  - Every source fails: (nil, error) with all source errors joined.
*/
func LoadGPUPools(ctx context.Context, repoPath string, env models.Environment) ([]models.GPUPool, error) {
	logger := logging.FromContext(ctx)
	sources := []struct {
		dir          string
		localName    string
		isOkeManaged bool
	}{
		{filepath.Join(repoPath, "shared_modules/instance_pools_config"), "env_instance_pools_config", false},
		{filepath.Join(repoPath, "shared_modules/cluster_networks_config"), "env_cluster_networks_config", false},
		{filepath.Join(repoPath, "shared_modules/oci_oke_nodepools_config"), "env_nodepools_config", true},
	}

	var (
		gpuPools []models.GPUPool
		errs     []error
	)
	for _, s := range sources {
		pools, err := loadGPUPools(ctx, s.dir, s.localName, s.isOkeManaged, env)
		if err != nil {
			logger.Warnw("skipping unresolved GPUPool source",
				"dir", s.dir, "local", s.localName, "error", err)
			errs = append(errs, fmt.Errorf("%s: %w", s.localName, err))
			continue
		}
		gpuPools = append(gpuPools, pools...)
	}

	logger.Debugw("loaded gpu pools",
		"pools", len(gpuPools), "sources", len(sources), "failed_sources", len(errs))

	switch {
	case len(errs) == 0:
		return gpuPools, nil
	case len(gpuPools) == 0:
		return nil, fmt.Errorf("failed to parse HCL file: %w", errors.Join(errs...))
	default:
		return gpuPools, &PartialLoadError{Source: "GPUPools", Errs: errs}
	}
}

func loadGPUPools(ctx context.Context, dirPath, poolConfigName string, isOkeManaged bool,
	env models.Environment,
) ([]models.GPUPool, error) {
	valueMap, err := loadLocalValueMap(ctx, dirPath, env)
	if err != nil {
		return nil, err
	}

	poolsValue, ok := valueMap[poolConfigName]
	if !ok {
		return nil, fmt.Errorf("node pools config %s not resolved", poolConfigName)
	}

	var gpuPools []models.GPUPool
	for name, value := range poolsValue.AsValueMap() {
		pool := models.GPUPool{Name: name, IsOkeManaged: isOkeManaged, CapacityType: "on-demand", Status: models.GPUPoolStatusPending}
		for k, v := range value.AsValueMap() {
			switch k {
			case "shape":
				pool.Shape = v.AsString()
			case "size", "node_pool_size":
				size, _ := v.AsBigFloat().Int64()
				pool.Size = int(size)
			case "capacity_type":
				pool.CapacityType = v.AsString()
			case "placement_logical_ad":
				pool.AvailabilityDomain = extractAvailabilityDomain(v)
			}
		}

		if strings.Contains(pool.Shape, "GPU") {
			gpuPools = append(gpuPools, pool)
		}
	}

	// cty map iteration order varies by run; without a sort the rows,
	// `get gpupool --limit` and a CSV export would change on every load.
	sort.Slice(gpuPools, func(i, j int) bool { return gpuPools[i].Name < gpuPools[j].Name })
	return gpuPools, nil
}

func extractAvailabilityDomain(v cty.Value) string {
	t := v.Type()
	if t.IsPrimitiveType() && t.FriendlyName() == "string" {
		return v.AsString()
	} else if t.IsListType() || t.IsTupleType() {
		if slice := v.AsValueSlice(); len(slice) > 0 {
			s := slice[0].AsString()
			if parts := strings.Split(s, "-AD-"); len(parts) == 2 {
				return "AD-" + parts[1]
			}
		}
	}
	return ""
}

// LoadModelArtifacts loads ModelArtifact objects from the given repository path and environment.
func LoadModelArtifacts(ctx context.Context, repoPath string, env models.Environment) (map[string][]models.ModelArtifact, error) {
	dirPath := filepath.Join(repoPath, "shared_modules/tensorrt_models_config")
	valueMap, err := loadLocalValueMap(ctx, dirPath, env)
	if err != nil {
		return nil, err
	}

	modelMapValue, ok := valueMap["all_models_map"]
	if !ok {
		return nil, fmt.Errorf("all_models_map: %w", ErrModelArtifactMapNotResolved)
	}

	modelArtifactMap := map[string][]models.ModelArtifact{}
	for modelName, value := range modelMapValue.AsValueMap() {
		modelArtifacts := make([]models.ModelArtifact, 0, 4)
		for trtVersion, v1 := range value.AsValueMap() {
			for gpuShape, v2 := range v1.AsValueMap() {
				for gpuCount, v3 := range v2.AsValueMap() {
					artifact := models.ModelArtifact{
						Name:            v3.AsString(),
						TensorRTVersion: trtVersion,
						GPUCount:        extractGPUNumber(gpuCount),
						GPUShape:        gpuShape,
						ModelName:       modelName,
					}

					modelArtifacts = append(modelArtifacts, artifact)
				}
			}
		}

		// cty map iteration order varies by run, so `get ma --limit`
		// would pick different rows each time. One artifact can serve
		// several GPU configs, so the name alone doesn't order them.
		slices.SortFunc(modelArtifacts, func(a, b models.ModelArtifact) int {
			return cmp.Or(
				strings.Compare(a.Name, b.Name),
				strings.Compare(a.GPUShape, b.GPUShape),
				cmp.Compare(a.GPUCount, b.GPUCount),
				strings.Compare(a.TensorRTVersion, b.TensorRTVersion),
			)
		})
		modelArtifactMap[modelName] = modelArtifacts
	}

	return modelArtifactMap, nil
}

func extractGPUNumber(s string) int {
	numStr := strings.TrimSuffix(s, "Gpu")
	num, _ := strconv.Atoi(numStr)
	return num
}

func createAvailabilityDomains() cty.Value {
	// data.oci_identity_availability_domains.ad_list.availability_domains
	ad1 := cty.ObjectVal(map[string]cty.Value{"name": cty.StringVal("AD-1")})
	ads := cty.ListVal([]cty.Value{ad1})
	adsValue := cty.ObjectVal(map[string]cty.Value{"availability_domains": ads})
	return cty.ObjectVal(map[string]cty.Value{"ad_list": adsValue})
}

func createObjectStorageNamespace() cty.Value {
	// data.oci_objectstorage_namespace.objectstorage_namespace.namespace
	ns := cty.ObjectVal(map[string]cty.Value{"namespace": cty.StringVal("NAMESPACE")})
	return cty.ObjectVal(map[string]cty.Value{"objectstorage_namespace": ns})
}
