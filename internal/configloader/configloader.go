/*
Package configloader provides utilities for loading configuration overrides and tenancy data for the toolkit.
*/
package configloader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	"github.com/jingle2008/toolkit/internal/fs"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	models "github.com/jingle2008/toolkit/pkg/models"
)

const (
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
	return loadOverridesDI(dirPath, fs.ListFiles, jsonutil.LoadFile[T])
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
	return loadTenancyOverridesDI(root, realm, name, listSubDirs, loadOverrides[T])
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

// LoadDataset loads a Dataset from the given repository path and environment.
// Now accepts context.Context as the first parameter.
func LoadDataset(ctx context.Context, repoPath string, env models.Environment) (*models.Dataset, error) {
	serviceTenancies, err := terraform.LoadServiceTenancies(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	environments := getEnvironments(serviceTenancies)
	if err := validateEnvironment(env, environments); err != nil {
		return nil, err
	}

	limitsRoot := filepath.Join(repoPath, "shared_modules/limits")
	realm := env.Realm

	limitGroup, consolePropertyDefinitionGroup, propertyDefinitionGroup, err := loadDefinitionGroups(limitsRoot, realm)
	if err != nil {
		return nil, err
	}

	tenants, limitTenancyOverrideMap, consolePropertyTenancyOverrideMap, propertyTenancyOverrideMap, err := buildTenantMap(limitsRoot, realm)
	if err != nil {
		return nil, err
	}

	consolePropertyRegionalOverrides, propertyRegionalOverrides, err := loadRegionalOverridesGroups(limitsRoot, realm)
	if err != nil {
		return nil, err
	}

	modelArtifacts, err := terraform.LoadModelArtifacts(ctx, repoPath, env)
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

// validateEnvironment checks if the provided environment is valid.
func validateEnvironment(env models.Environment, allEnvs []models.Environment) error {
	if !isValidEnvironment(env, allEnvs) {
		return errors.New("environment is not valid or in the list")
	}
	return nil
}

// loadDefinitionGroups loads and sorts the definition groups.
func loadDefinitionGroups(limitsRoot, realm string) (
	*models.LimitDefinitionGroup,
	*models.ConsolePropertyDefinitionGroup,
	*models.PropertyDefinitionGroup,
	error,
) {
	limitDefinitionPath := getConfigPath(limitsRoot, realm, limitsKey+definitionSuffix)
	limitGroup, err := jsonutil.LoadFile[models.LimitDefinitionGroup](limitDefinitionPath)
	if err != nil {
		return nil, nil, nil, err
	}
	sortNamedItems(limitGroup.Values)

	consolePropertyDefinitionPath := getConfigPath(limitsRoot, realm, consolePropertiesKey+definitionSuffix)
	consolePropertyDefinitionGroup, err := jsonutil.LoadFile[models.ConsolePropertyDefinitionGroup](consolePropertyDefinitionPath)
	if err != nil {
		return nil, nil, nil, err
	}
	sortNamedItems(consolePropertyDefinitionGroup.Values)

	propertyDefinitionPath := getConfigPath(limitsRoot, realm, propertiesKey+definitionSuffix)
	propertyDefinitionGroup, err := jsonutil.LoadFile[models.PropertyDefinitionGroup](propertyDefinitionPath)
	if err != nil {
		return nil, nil, nil, err
	}
	sortNamedItems(propertyDefinitionGroup.Values)

	return limitGroup, consolePropertyDefinitionGroup, propertyDefinitionGroup, nil
}

// buildTenantMap builds the tenant map and returns tenants and override maps.
func buildTenantMap(limitsRoot, realm string) (
	[]models.Tenant,
	map[string][]models.LimitTenancyOverride,
	map[string][]models.ConsolePropertyTenancyOverride,
	map[string][]models.PropertyTenancyOverride,
	error,
) {
	tenantMap := make(map[string]tenantInfo)

	limitTenancyOverrideMap, err := loadTenancyOverrides[models.LimitTenancyOverride](
		limitsRoot, realm, limitsKey+tenancyOverridesKey)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	updateTenants(tenantMap, limitTenancyOverrideMap, 0)

	consolePropertyTenancyOverrideMap, err := loadTenancyOverrides[models.ConsolePropertyTenancyOverride](
		limitsRoot, realm, consolePropertiesKey+tenancyOverridesKey)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	updateTenants(tenantMap, consolePropertyTenancyOverrideMap, 1)

	propertyTenancyOverrideMap, err := loadTenancyOverrides[models.PropertyTenancyOverride](
		limitsRoot, realm, propertiesKey+tenancyOverridesKey)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	updateTenants(tenantMap, propertyTenancyOverrideMap, 2)

	tenants := getTenants(tenantMap)
	return tenants, limitTenancyOverrideMap, consolePropertyTenancyOverrideMap, propertyTenancyOverrideMap, nil
}

// loadRegionalOverridesGroups loads the regional overrides for console property and property.
func loadRegionalOverridesGroups(limitsRoot, realm string) (
	[]models.ConsolePropertyRegionalOverride,
	[]models.PropertyRegionalOverride,
	error,
) {
	consolePropertyRegionalOverrides, err := loadRegionalOverrides[models.ConsolePropertyRegionalOverride](
		limitsRoot, realm, consolePropertiesKey+regionalOverridesKey)
	if err != nil {
		return nil, nil, err
	}

	propertyRegionalOverrides, err := loadRegionalOverrides[models.PropertyRegionalOverride](
		limitsRoot, realm, propertiesKey+regionalOverridesKey)
	if err != nil {
		return nil, nil, err
	}

	return consolePropertyRegionalOverrides, propertyRegionalOverrides, nil
}
