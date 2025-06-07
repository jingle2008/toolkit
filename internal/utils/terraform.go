package utils

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	models "github.com/jingle2008/toolkit/pkg/models"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
	"gopkg.in/yaml.v2"
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
}

func getLocalAttributesDI(
	dirPath string,
	listFilesFunc func(string, string) ([]string, error),
	updateLocalAttributesFunc func(string, hclsyntax.Attributes) error,
) (hclsyntax.Attributes, error) {
	tfFiles, err := listFilesFunc(dirPath, ".tf")
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	attributes := make(hclsyntax.Attributes)
	for _, file := range tfFiles {
		if err := updateLocalAttributesFunc(file, attributes); err != nil {
			return nil, fmt.Errorf("failed to update local attributes: %w", err)
		}
	}

	return attributes, nil
}

func getLocalAttributes(dirPath string) (hclsyntax.Attributes, error) {
	return getLocalAttributesDI(dirPath, ListFiles, updateLocalAttributes)
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
			for key, value := range block.Body.Attributes {
				attributes[key] = value
			}
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

func loadLocalValueMap(dirPath string, env models.Environment) (map[string]cty.Value, error) {
	attributes, err := getLocalAttributes(dirPath)
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

	sort.Slice(keys, func(i, j int) bool {
		vi := len(attributes[keys[i]].Expr.Variables())
		vj := len(attributes[keys[j]].Expr.Variables())
		return vi < vj
	})

	progress := true
	for len(attributes) > 0 && progress {
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
	}

	for key := range attributes {
		attr, ok := attributes[key]
		if !ok {
			continue
		}

		value, diags := attr.Expr.Value(&context)
		if diags.HasErrors() {
			log.Println("can't resolve local:", key, ", errors:")
			for _, err := range diags.Errs() {
				log.Println("\t", err.Error())
			}

			continue
		}

		log.Println("can't resolve local:", key, ", value:", value)
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
func LoadServiceTenancies(_ context.Context, repoPath string) ([]models.ServiceTenancy, error) {
	dirPath := filepath.Join(repoPath, "shared_modules/shep_targets")
	attributes, err := getLocalAttributes(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse attributes: %w", err)
	}

	tenancyMap := make(map[string]*models.ServiceTenancy)

	for key, attribute := range attributes {
		// TODO: group_ locals don't resolve
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

	sortKeyedItems(tenancies)
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

func loadChartValuesMap(repoPath string) (map[string]*models.ChartValues, error) {
	dirPath := filepath.Join(repoPath, "model-serving/application/generic_region/model_chart_values")
	files, err := ListFiles(dirPath, ".yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to parse HCL file: %w", err)
	}

	result := make(map[string]*models.ChartValues)
	for _, file := range files {
		yamlData, err := SafeReadFile(
			file,
			dirPath,
			map[string]struct{}{".yaml": {}, ".yml": {}},
		)
		if err != nil {
			log.Println("failed to read chart values", err)
			continue
		}

		var config ChartValues
		err = yaml.Unmarshal(yamlData, &config)
		if err != nil {
			log.Println("failed to parse chart values", err)
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
func LoadBaseModels(ctx context.Context, repoPath string, env models.Environment) (
	map[string]*models.BaseModel, error,
) {
	dirPath := filepath.Join(repoPath, "model-serving/application/generic_region")
	locals, err := loadLocalValueMap(dirPath, env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HCL file: %w", err)
	}

	modelCapsValue, ok := locals["enabled_map"]
	if !ok {
		return nil, errors.New("base model map not resolved")
	}
	modelCapsMap := loadModelCapabilities(modelCapsValue)

	modelReplicasValue, ok := locals["regional_replica_map"]
	if !ok {
		return nil, errors.New("replica map not resolved")
	}
	modelReplicasMap := loadModelReplicas(modelReplicasValue)

	modelsValue, ok := locals["base_model_map"]
	if !ok {
		return nil, errors.New("base model map not resolved")
	}

	chartValuesMap, err := loadChartValuesMap(repoPath)
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

		model := getBaseModel(value, capabilities, chartValuesMap)
		for _, capability := range model.Capabilities {
			if replicas, ok := modelReplicasMap[capability.CrName]; ok {
				capability.Replicas = replicas
			}
		}

		baseModels[key] = model
	}

	deprecationValue, ok := locals["deprecation_map"]
	if !ok {
		return nil, errors.New("deprecation map not resolved")
	}
	updateModelLifecycle(baseModels, deprecationValue)

	return baseModels, nil
}

func getBaseModel(object cty.Value, enabledCaps map[string]struct{},
	chartValues map[string]*models.ChartValues,
) *models.BaseModel {
	result := models.BaseModel{}
	capabilities := make(map[string]*models.Capability)

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
				capabilities[name] = getCapability(value, chartValues)
			}
		}
	}

	result.Capabilities = capabilities
	return &result
}

func getCapability(object cty.Value, chartValues map[string]*models.ChartValues) *models.Capability {
	result := models.Capability{}

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
	pools, err := loadGpuPools(dirPath, "env_instance_pools_config", false, env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HCL file: %w", err)
	}

	gpuPools = append(gpuPools, pools...)

	// self-managed pools (cluster network)
	dirPath = filepath.Join(repoPath, "shared_modules/cluster_networks_config")
	pools, err = loadGpuPools(dirPath, "env_cluster_networks_config", false, env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HCL file: %w", err)
	}

	gpuPools = append(gpuPools, pools...)

	// oke-managed pools
	dirPath = filepath.Join(repoPath, "shared_modules/oci_oke_nodepools_config")
	pools, err = loadGpuPools(dirPath, "env_nodepools_config", true, env)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HCL file: %w", err)
	}

	gpuPools = append(gpuPools, pools...)

	sortNamedItems(gpuPools)
	return gpuPools, nil
}

func loadGpuPools(dirPath, poolConfigName string, isOkeManaged bool,
	env models.Environment,
) ([]models.GpuPool, error) {
	valueMap, err := loadLocalValueMap(dirPath, env)
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
func LoadModelArtifacts(ctx context.Context, repoPath string, env models.Environment) ([]models.ModelArtifact, error) {
	dirPath := filepath.Join(repoPath, "shared_modules/tensorrt_models_config")
	valueMap, err := loadLocalValueMap(dirPath, env)
	if err != nil {
		return nil, err
	}

	modelMapValue, ok := valueMap["all_models_map"]
	if !ok {
		return nil, errors.New("model artifact map not resolved")
	}

	modelArtifacts := []models.ModelArtifact{}
	for modelName, value := range modelMapValue.AsValueMap() {
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
	}

	sortKeyedItems(modelArtifacts)
	return modelArtifacts, nil
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
