/*
Package utils provides utility functions for configuration management.
*/
package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	models "github.com/jingle2008/toolkit/pkg/models"
)

const (
	// keys for config sections
	limitsKey            = "limits"
	propertiesKey        = "properties"
	consolePropertiesKey = "console_properties"
	definitionSuffix     = "_definition"
	tenancyOverridesKey  = "_tenancy_overrides"
	regionalOverridesKey = "_regional_overrides"
	regionalValuesDir    = "regional_values"
)

func getConfigPath(root, realm, configName string) string {
	configFile := fmt.Sprintf("%s_%s.json", realm, configName)
	return filepath.Join(root, configName+"s", configFile)
}

func listSubDirs(dirPath string) ([]string, error) {
	var subDirs []string

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDirs = append(subDirs, entry.Name())
		}
	}

	return subDirs, nil
}

func loadOverridesDI[T models.NamedItem](
	dirPath string,
	listFilesFunc func(string, string) ([]string, error),
	loadFileFunc func(string) (*T, error),
) ([]T, error) {
	overrideFiles, err := listFilesFunc(dirPath, ".json")
	if err != nil {
		return nil, err
	}

	overrides := make([]T, 0, len(overrideFiles))
	for _, file := range overrideFiles {
		override, err := loadFileFunc(file)
		if err != nil {
			return nil, err
		}
		overrides = append(overrides, *override)
	}

	sortNamedItems(overrides)
	return overrides, nil
}

func loadOverrides[T models.NamedItem](dirPath string) ([]T, error) {
	return loadOverridesDI[T](dirPath, ListFiles, LoadFile[T])
}

func loadTenancyOverridesDI[T models.NamedItem](
	root, realm, name string,
	listSubDirsFunc func(string) ([]string, error),
	loadOverridesFunc func(string) ([]T, error),
) (map[string][]T, error) {
	results := make(map[string][]T)

	realmDir := filepath.Join(root, name, regionalValuesDir, realm)
	tenants, err := listSubDirsFunc(realmDir)
	if err != nil {
		return nil, err
	}

	for _, tenant := range tenants {
		overrides, err := loadOverridesFunc(filepath.Join(realmDir, tenant))
		if err != nil {
			return nil, err
		}
		results[tenant] = overrides
	}

	return results, nil
}

func loadTenancyOverrides[T models.NamedItem](root, realm, name string) (map[string][]T, error) {
	return loadTenancyOverridesDI[T](root, realm, name, listSubDirs, loadOverrides[T])
}

func loadRegionalOverrides[T models.NamedItem](root, realm, name string) ([]T, error) {
	realmDir := filepath.Join(root, name, regionalValuesDir, realm)
	overrides, err := loadOverrides[T](realmDir)
	if err != nil {
		return nil, err
	}

	return overrides, nil
}

func sortNamedItems[T models.NamedItem](items []T) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].GetName() < items[j].GetName()
	})
}

func sortKeyedItems[T models.KeyedItem](items []T) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].GetKey() < items[j].GetKey()
	})
}

type tenantInfo struct {
	idMap     map[string]struct{}
	overrides []int
}

func getTenants(tenantMap map[string]tenantInfo) []models.Tenant {
	tenants := make([]models.Tenant, 0, len(tenantMap))

	lo, cpo, po := 0, 0, 0
	for k, v := range tenantMap {
		ids := make([]string, 0, len(v.idMap))
		for k := range v.idMap {
			ids = append(ids, k)
		}
		tenant := models.Tenant{
			IDs:                      ids,
			Name:                     k,
			LimitOverrides:           v.overrides[0],
			ConsolePropertyOverrides: v.overrides[1],
			PropertyOverrides:        v.overrides[2],
		}
		tenants = append(tenants, tenant)

		lo += v.overrides[0]
		cpo += v.overrides[1]
		po += v.overrides[2]
	}

	sortNamedItems(tenants)
	return tenants
}

func updateTenants[T models.TenancyOverride](
	tenantMap map[string]tenantInfo, overrideMap map[string][]T, index int,
) {
	for name, overrides := range overrideMap {
		info, ok := tenantMap[name]
		if !ok {
			info = tenantInfo{
				idMap:     make(map[string]struct{}),
				overrides: make([]int, 3),
			}
			tenantMap[name] = info
		}

		for _, o := range overrides {
			tenantID := o.GetTenantID()
			info.idMap[tenantID] = struct{}{}
			info.overrides[index]++
		}
	}
}

func getEnvironments(tenancies []models.ServiceTenancy) []models.Environment {
	var environments []models.Environment
	for _, t := range tenancies {
		environments = append(environments, t.Environments()...)
	}

	sortKeyedItems(environments)
	return environments
}

func isValidEnvironment(env models.Environment, allEnvs []models.Environment) bool {
	for _, e := range allEnvs {
		if e.Equals(env) {
			return true
		}
	}

	return false
}

/*
LoadDataset loads a Dataset from the given repository path and environment.
*/
func LoadDataset(repoPath string, env models.Environment) (*models.Dataset, error) {
	serviceTenancies, err := LoadServiceTenancies(repoPath)
	if err != nil {
		return nil, err
	}

	environments := getEnvironments(serviceTenancies)
	if !isValidEnvironment(env, environments) {
		return nil, errors.New("environment is not valid or in the list")
	}

	limitsRoot := filepath.Join(repoPath, "shared_modules/limits")

	realm := env.Realm
	limitDefinitionPath := getConfigPath(limitsRoot, realm, limitsKey+definitionSuffix)
	limitGroup, err := LoadFile[models.LimitDefinitionGroup](limitDefinitionPath)
	if err != nil {
		return nil, err
	}

	sortNamedItems(limitGroup.Values)

	consolePropertyDefinitionPath := getConfigPath(limitsRoot, realm, consolePropertiesKey+definitionSuffix)
	consolePropertyDefinitionGroup, err := LoadFile[models.ConsolePropertyDefinitionGroup](consolePropertyDefinitionPath)
	if err != nil {
		return nil, err
	}

	sortNamedItems(consolePropertyDefinitionGroup.Values)

	propertyDefinitionPath := getConfigPath(limitsRoot, realm, propertiesKey+definitionSuffix)
	propertyDefinitionGroup, err := LoadFile[models.PropertyDefinitionGroup](propertyDefinitionPath)
	if err != nil {
		return nil, err
	}

	sortNamedItems(propertyDefinitionGroup.Values)

	tenantMap := make(map[string]tenantInfo)

	limitTenancyOverrideMap, err := loadTenancyOverrides[models.LimitTenancyOverride](
		limitsRoot, realm, limitsKey+tenancyOverridesKey)
	if err != nil {
		return nil, err
	}

	updateTenants(tenantMap, limitTenancyOverrideMap, 0)

	consolePropertyTenancyOverrideMap, err := loadTenancyOverrides[models.ConsolePropertyTenancyOverride](
		limitsRoot, realm, consolePropertiesKey+tenancyOverridesKey)
	if err != nil {
		return nil, err
	}

	updateTenants(tenantMap, consolePropertyTenancyOverrideMap, 1)

	propertyTenancyOverrideMap, err := loadTenancyOverrides[models.PropertyTenancyOverride](
		limitsRoot, realm, propertiesKey+tenancyOverridesKey)
	if err != nil {
		return nil, err
	}

	updateTenants(tenantMap, propertyTenancyOverrideMap, 2)
	tenants := getTenants(tenantMap)

	consolePropertyRegionalOverrides, err := loadRegionalOverrides[models.ConsolePropertyRegionalOverride](
		limitsRoot, realm, consolePropertiesKey+regionalOverridesKey)
	if err != nil {
		return nil, err
	}

	propertyRegionalOverrides, err := loadRegionalOverrides[models.PropertyRegionalOverride](
		limitsRoot, realm, propertiesKey+regionalOverridesKey)
	if err != nil {
		return nil, err
	}

	modelArtifacts, err := LoadModelArtifacts(repoPath, env)
	if err != nil {
		return nil, err
	}

	return &models.Dataset{
		LimitDefinitionGroup:              *limitGroup,
		ConsolePropertyDefinitionGroup:    *consolePropertyDefinitionGroup,
		PropertyDefinitionGroup:           *propertyDefinitionGroup,
		LimitTenancyOverrideMap:           limitTenancyOverrideMap,
		ConsolePropertyTenancyOverrideMap: consolePropertyTenancyOverrideMap,
		PropertyTenancyOverrideMap:        propertyTenancyOverrideMap,
		ConsolePropertyRegionalOverrides:  consolePropertyRegionalOverrides,
		PropertyRegionalOverrides:         propertyRegionalOverrides,
		Tenants:                           tenants,
		Environments:                      environments,
		ServiceTenancies:                  serviceTenancies,
		ModelArtifacts:                    modelArtifacts,
	}, nil
}
