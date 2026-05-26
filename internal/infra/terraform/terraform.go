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
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
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
	outputBlock = "output"
	outputValue = "value"
)

var localFuncMap = map[string]function.Function{
	"format":   stdlib.FormatFunc,
	"lookup":   stdlib.LookupFunc,
	"merge":    stdlib.MergeFunc,
	"join":     stdlib.JoinFunc,
	"contains": stdlib.ContainsFunc,
	"keys":     stdlib.KeysFunc,
	"flatten":  stdlib.FlattenFunc,
	"distinct": stdlib.DistinctFunc,
}

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
	valueMap[key] = value
	return cty.ObjectVal(valueMap)
}

func loadLocalValueMap(ctx context.Context, dirPath string, env models.Environment) (map[string]cty.Value, error) { //nolint:cyclop
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
		},
		Functions: localFuncMap,
	}

	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}

	slices.SortFunc(keys, func(a, b string) int {
		vi := len(attributes[a].Expr.Variables())
		vj := len(attributes[b].Expr.Variables())
		return cmp.Compare(vi, vj)
	})

	const maxIterations = 100
	progress := true
	iterations := 0
	for len(attributes) > 0 && progress && iterations < maxIterations {
		knownKeys := make(map[string]struct{})
		for _, key := range keys {
			attr, ok := attributes[key]
			if !ok {
				continue
			}

			value, diags := attr.Expr.Value(&context)
			if diags.HasErrors() {
				continue
			}

			if value.IsWhollyKnown() {
				localObject = mergeObject(localObject, key, value)
				context.Variables[localKey] = localObject
				knownKeys[key] = struct{}{}
			}
		}

		progress = len(knownKeys) > 0
		for key := range knownKeys {
			delete(attributes, key)
		}
		iterations++
	}
	if iterations == maxIterations && len(attributes) > 0 {
		logger.Errorw("max iterations reached while resolving locals; possible cyclic dependency", "unresolved", keys)
	}

	for key := range attributes {
		attr, ok := attributes[key]
		if !ok {
			continue
		}

		value, diags := attr.Expr.Value(&context)
		if diags.HasErrors() {
			logger.Errorw("cannot resolve local",
				"local", key,
				"errors", diags.Errs(),
			)
			continue
		}

		logger.Infow("cannot resolve local value",
			"local", key,
			"value", value.GoString(),
		)
	}

	return localObject.AsValueMap(), nil
}

// LoadServiceTenancies loads ServiceTenancy objects from the given repository path.
func LoadServiceTenancies(ctx context.Context, repoPath string) ([]models.ServiceTenancy, error) {
	dirPath := filepath.Join(repoPath, "shared_modules/shep_targets")
	attributes, err := LoadLocalAttributes(ctx, dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse attributes: %w", err)
	}

	tenancyMap := make(map[string]*models.ServiceTenancy)

	for key, attribute := range attributes {
		if key == "tenancy_name_mapping" ||
			strings.HasPrefix(key, "group_") ||
			key == "region_groups" {
			continue
		}

		realm := strings.Split(key, "_")[0]
		value, diags := attribute.Expr.Value(nil)
		if diags.HasErrors() {
			return nil, errors.New(diags.Error())
		}

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

	return tenancies, nil
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
			var regions []string
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
			logger.Errorw("skipping unresolved GPUPool source",
				"dir", s.dir, "local", s.localName, "error", err)
			errs = append(errs, fmt.Errorf("%s: %w", s.localName, err))
			continue
		}
		gpuPools = append(gpuPools, pools...)
	}

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
		pool := models.GPUPool{Name: name, IsOkeManaged: isOkeManaged, CapacityType: "on-demand", Status: "..."}
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
