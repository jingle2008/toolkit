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
	fs "github.com/jingle2008/toolkit/internal/fileutil"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	models "github.com/jingle2008/toolkit/pkg/models"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
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
GetLocalAttributes loads and returns all local attributes from Terraform files in the specified directory.
*/
func GetLocalAttributes(ctx context.Context, dirPath string) (hclsyntax.Attributes, error) {
	return getLocalAttributesDI(ctx, dirPath, fs.ListFiles, updateLocalAttributes)
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

func mergeObject(object cty.Value, key string, value cty.Value) cty.Value {
	valueMap := object.AsValueMap()
	valueMap[key] = value
	return cty.ObjectVal(valueMap)
}

func loadLocalValueMap(ctx context.Context, dirPath string, env models.Environment) (map[string]cty.Value, error) { //nolint:cyclop
	logger := logging.FromContext(ctx)
	attributes, err := GetLocalAttributes(ctx, dirPath)
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

	varObject := cty.ObjectVal(map[string]cty.Value{
		"region":      cty.StringVal(env.Region),
		"environment": cty.StringVal(env.Type),
	})

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

/*
LoadServiceTenancies loads ServiceTenancy objects from the given repository path.
Now accepts context.Context as the first parameter.
*/
func LoadServiceTenancies(ctx context.Context, repoPath string) ([]models.ServiceTenancy, error) {
	dirPath := filepath.Join(repoPath, "shared_modules/shep_targets")
	attributes, err := GetLocalAttributes(ctx, dirPath)
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
LoadGpuPools loads GpuPool objects from the given repository path and environment.
Now accepts context.Context as the first parameter.
*/
func LoadGpuPools(ctx context.Context, repoPath string, env models.Environment) ([]models.GpuPool, error) {
	var gpuPools []models.GpuPool

	// self-managed pools
	dirPath := filepath.Join(repoPath, "shared_modules/instance_pools_config")
	pools, err := loadGpuPools(ctx, dirPath, "env_instance_pools_config", false, env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HCL file: %w", err)
	}

	gpuPools = append(gpuPools, pools...)

	// self-managed pools (cluster network)
	dirPath = filepath.Join(repoPath, "shared_modules/cluster_networks_config")
	pools, err = loadGpuPools(ctx, dirPath, "env_cluster_networks_config", false, env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HCL file: %w", err)
	}

	gpuPools = append(gpuPools, pools...)

	// oke-managed pools
	dirPath = filepath.Join(repoPath, "shared_modules/oci_oke_nodepools_config")
	pools, err = loadGpuPools(ctx, dirPath, "env_nodepools_config", true, env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HCL file: %w", err)
	}

	gpuPools = append(gpuPools, pools...)
	return gpuPools, nil
}

func loadGpuPools(ctx context.Context, dirPath, poolConfigName string, isOkeManaged bool,
	env models.Environment,
) ([]models.GpuPool, error) {
	valueMap, err := loadLocalValueMap(ctx, dirPath, env)
	if err != nil {
		return nil, err
	}

	poolsValue, ok := valueMap[poolConfigName]
	if !ok {
		return nil, fmt.Errorf("node pools config %s not resolved", poolConfigName)
	}

	var gpuPools []models.GpuPool
	for name, value := range poolsValue.AsValueMap() {
		pool := models.GpuPool{Name: name, IsOkeManaged: isOkeManaged, CapacityType: "on-demand", Status: "..."}
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

/*
LoadModelArtifacts loads ModelArtifact objects from the given repository path and environment.
Now accepts context.Context as the first parameter.
*/
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
						GpuCount:        extractGpuNumber(gpuCount),
						GpuShape:        gpuShape,
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

func extractGpuNumber(s string) int {
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
