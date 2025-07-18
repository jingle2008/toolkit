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
	"gopkg.in/yaml.v3"
)

var (
	// ErrBaseModelMapNotResolved is returned when the base model map cannot be resolved.
	ErrBaseModelMapNotResolved = errors.New("base model map not resolved")
	// ErrReplicaMapNotResolved is returned when the replica map cannot be resolved.
	ErrReplicaMapNotResolved = errors.New("replica map not resolved")
	// ErrDeprecationMapNotResolved is returned when the deprecation map cannot be resolved.
	ErrDeprecationMapNotResolved = errors.New("deprecation map not resolved")
	// ErrModelArtifactMapNotResolved is returned when the model artifact map cannot be resolved.
	ErrModelArtifactMapNotResolved = errors.New("model artifact map not resolved")
)

/*
ChartValues represents the values used for chart templating in model deployment.

Fields:
  - Model: model settings for deployment
  - ModelMetaData: metadata including DAC shape configs, training configs, and serving base model configs
*/
type ChartValues struct {
	Model         *models.ModelSetting `yaml:"model"`
	ModelMetaData *struct {
		DacShapeConfigs         *string `yaml:"dacShapeConfigs"`
		TrainingConfigs         *string `yaml:"trainingConfigs"`
		ServingBaseModelConfigs *string `yaml:"servingBaseModelConfigs"`
	} `yaml:"modelMetaData"`
}

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

func getLocalAttributes(ctx context.Context, dirPath string) (hclsyntax.Attributes, error) {
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
	attributes, err := getLocalAttributes(ctx, dirPath)
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

func loadModelCapabilities(object cty.Value) map[string]map[string]struct{} {
	modelCapsMap := make(map[string]map[string]struct{})

	for modelID, value := range object.AsValueMap() {
		capabilities := make(map[string]struct{})
		for _, capValue := range value.AsValueSlice() {
			capabilities[capValue.AsString()] = struct{}{}
		}

		modelCapsMap[modelID] = capabilities
	}

	return modelCapsMap
}

func updateModelLifecycle(models map[string]*models.BaseModel, stateObject cty.Value) {
	for modelID, state := range stateObject.AsValueMap() {
		model, ok := models[modelID]
		if !ok {
			continue
		}

		for name, value := range state.AsValueMap() {
			switch name {
			case "baseModelLifeCyclePhase":
				model.LifeCyclePhase = value.AsString()
			case "timeDeprecated":
				model.TimeDeprecated = value.AsString()
			}
		}
	}
}

func loadModelReplicas(object cty.Value) map[string]int {
	modelReplicaMap := make(map[string]int)

	for crName, value := range object.AsValueMap() {
		replicas, _ := value.AsBigFloat().Int64()
		modelReplicaMap[crName] = int(replicas)
	}

	return modelReplicaMap
}

/*
LoadServiceTenancies loads ServiceTenancy objects from the given repository path.
Now accepts context.Context as the first parameter.
*/
func LoadServiceTenancies(ctx context.Context, repoPath string) ([]models.ServiceTenancy, error) {
	dirPath := filepath.Join(repoPath, "shared_modules/shep_targets")
	attributes, err := getLocalAttributes(ctx, dirPath)
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

func loadChartValuesMap(ctx context.Context, repoPath string) (map[string]*models.ChartValues, error) {
	logger := logging.FromContext(ctx)
	dirPath := filepath.Join(repoPath, "model-serving/application/generic_region/model_chart_values")
	files, err := fs.ListFiles(ctx, dirPath, ".yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to parse HCL file: %w", err)
	}

	result := make(map[string]*models.ChartValues)
	for _, file := range files {
		yamlData, err := fs.SafeReadFile(
			file,
			dirPath,
			map[string]struct{}{".yaml": {}, ".yml": {}},
		)
		if err != nil {
			logger.Errorw("failed to read chart values", "error", err)
			continue
		}

		var config ChartValues
		err = yaml.Unmarshal(yamlData, &config)
		if err != nil {
			logger.Errorw("failed to parse chart values", "error", err)
			continue
		}
		result[filepath.Base(file)] = convertChartValues(config)
	}

	return result, nil
}

func convertChartValues(values ChartValues) *models.ChartValues {
	var modelMetaData *models.ModelMetaData

	if values.ModelMetaData != nil {
		dacShapeConfigs := unmarshalYaml[models.DacShapeConfigs](
			values.ModelMetaData.DacShapeConfigs)
		trainingConfigs := unmarshalYaml[models.TrainingConfigs](
			values.ModelMetaData.TrainingConfigs)
		servingBaseModelConfigs := unmarshalYaml[models.ServingBaseModelConfigs](
			values.ModelMetaData.ServingBaseModelConfigs)

		modelMetaData = &models.ModelMetaData{
			DacShapeConfigs:         dacShapeConfigs,
			TrainingConfigs:         trainingConfigs,
			ServingBaseModelConfigs: servingBaseModelConfigs,
		}
	}

	return &models.ChartValues{
		Model:         values.Model,
		ModelMetaData: modelMetaData,
	}
}

func unmarshalYaml[T any](text *string) *T {
	if text == nil {
		return nil
	}

	var result T
	err := yaml.Unmarshal([]byte(*text), &result)
	if err != nil {
		return nil
	}
	return &result
}

/*
LoadBaseModels loads base model definitions from the given repository path and environment.
Now accepts context.Context as the first parameter.
*/
func LoadBaseModels(ctx context.Context, repoPath string, env models.Environment) ( //nolint:cyclop
	map[string]*models.BaseModel, error,
) {
	dirPath := filepath.Join(repoPath, "model-serving/application/generic_region")
	locals, err := loadLocalValueMap(ctx, dirPath, env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HCL file: %w", err)
	}

	modelCapsValue, ok := locals["enabled_map"]
	if !ok {
		return nil, fmt.Errorf("enabled_map: %w", ErrBaseModelMapNotResolved)
	}
	modelCapsMap := loadModelCapabilities(modelCapsValue)

	modelReplicasValue, ok := locals["regional_replica_map"]
	if !ok {
		return nil, fmt.Errorf("regional_replica_map: %w", ErrReplicaMapNotResolved)
	}
	modelReplicasMap := loadModelReplicas(modelReplicasValue)

	modelsValue, ok := locals["base_model_map"]
	if !ok {
		return nil, fmt.Errorf("base_model_map: %w", ErrBaseModelMapNotResolved)
	}

	chartValuesMap, err := loadChartValuesMap(ctx, repoPath)
	if err != nil {
		return nil, errors.New("failed to load chart values")
	}

	// base models per env
	baseModels := make(map[string]*models.BaseModel)
	for key, value := range modelsValue.AsValueMap() {
		capabilities, ok := modelCapsMap[key]
		if !ok || len(capabilities) == 0 {
			continue
		}

		model := getBaseModel(ctx, value, capabilities, chartValuesMap)
		for _, capability := range model.Capabilities {
			if replicas, ok := modelReplicasMap[capability.CrName]; ok {
				capability.Replicas = replicas
			}
		}

		baseModels[key] = model
	}

	deprecationValue, ok := locals["deprecation_map"]
	if !ok {
		return nil, fmt.Errorf("deprecation_map: %w", ErrDeprecationMapNotResolved)
	}
	updateModelLifecycle(baseModels, deprecationValue)

	return baseModels, nil
}

//nolint:cyclop
func getBaseModel(ctx context.Context, object cty.Value, enabledCaps map[string]struct{},
	chartValues map[string]*models.ChartValues,
) *models.BaseModel {
	result := models.BaseModel{}
	capabilities := make(map[string]*models.Capability)

	logger := logging.FromContext(ctx)
	for name, value := range object.AsValueMap() {
		switch name {
		case "internal_name":
			result.InternalName = value.AsString()
		case "displayName":
			result.Name = value.AsString()
		case "type":
			result.Type = value.AsString()
		case "category":
			result.Category = value.AsString()
		case "version":
			result.Version = value.AsString()
		case "vendor":
			result.Vendor = value.AsString()
		case "maxTokens":
			v, _ := value.AsBigFloat().Int64()
			result.MaxTokens = int(v)
		case "vaultKey":
			result.VaultKey = value.AsString()
		case "isExperimental":
			result.IsExperimental = value.True()
		case "isInternal":
			result.IsInternal = value.True()
		case "isLongTermSupported":
			result.IsLongTermSupported = value.True()
		case "generation", "summarization", "chat", "embedding", "rerank":
			if _, ok := enabledCaps[name]; ok {
				capabilities[name] = getCapability(ctx, value, chartValues)
			}
		case "imageTextToText":
			v := value.True()
			result.ImageTextToText = &v
		case "containerImageOverride":
			v := value.AsString()
			result.ContainerImageOverride = &v
		default:
			logger.Errorw("unknown base model attribute", "name", name)
		}
	}

	result.Capabilities = capabilities
	return &result
}

func getCapability(ctx context.Context, object cty.Value, chartValues map[string]*models.ChartValues) *models.Capability {
	result := models.Capability{}

	logger := logging.FromContext(ctx)
	for name, value := range object.AsValueMap() {
		switch name {
		case "capability":
			result.Capability = value.AsString()
		case "cr_name":
			result.CrName = value.AsString()
		case "description":
			result.Description = value.AsString()
		case "runtime":
			result.Runtime = value.AsString()
		case "values_file":
			file := value.AsString()
			result.ValuesFile = &file
			chartName := filepath.Base(file)
			result.ChartValues = chartValues[chartName]
		case "max_loading_seconds":
			v := value.AsString()
			result.MaxLoadingSeconds = &v
		default:
			logger.Errorw("unknown capability attribute", "name", name)
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
		pool := models.GpuPool{Name: name, IsOkeManaged: isOkeManaged, CapacityType: "on-demand"}
		for k, v := range value.AsValueMap() {
			switch k {
			case "shape":
				pool.Shape = v.AsString()
			case "size", "node_pool_size":
				size, _ := v.AsBigFloat().Int64()
				pool.Size = int(size)
			case "capacity_type":
				pool.CapacityType = v.AsString()
			}
		}

		if strings.Contains(pool.Shape, "GPU") {
			gpuPools = append(gpuPools, pool)
		}
	}

	return gpuPools, nil
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
