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
	LIMITS             = "limits"
	PROPERTIES         = "properties"
	CONSOLE_PROPERTIES = "console_properties"
	DEFINITION         = "_definition"
	TENANCY_OVERRIDES  = "_tenancy_overrides"
	REGIONAL_OVERRIDES = "_regional_overrides"
	REGIONAL_VALUES    = "regional_values"
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

func loadOverrides[T models.NamedItem](dirPath string) ([]T, error) {
	overrideFiles, err := ListFiles(dirPath, ".json")
	if err != nil {
		return nil, err
	}

	overrides := make([]T, 0, len(overrideFiles))
	for _, file := range overrideFiles {
		override, err := LoadFile[T](file)
		if err != nil {
			return nil, err
		}

		overrides = append(overrides, *override)
	}

	sortNamedItems(overrides)
	return overrides, nil
}

func loadTenancyOverrides[T models.NamedItem](root, realm, name string) (map[string][]T, error) {
	results := make(map[string][]T)

	realmDir := filepath.Join(root, name, REGIONAL_VALUES, realm)
	tenants, err := listSubDirs(realmDir)
	if err != nil {
		return nil, err
	}

	for _, tenant := range tenants {
		overrides, err := loadOverrides[T](filepath.Join(realmDir, tenant))
		if err != nil {
			return nil, err
		}

		results[tenant] = overrides
	}

	return results, nil
}

func loadRegionalOverrides[T models.NamedItem](root, realm, name string) ([]T, error) {
	realmDir := filepath.Join(root, name, REGIONAL_VALUES, realm)
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
			Ids:                      ids,
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
			tenantId := o.GetTenantId()
			info.idMap[tenantId] = struct{}{}
			info.overrides[index] += 1
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
	limitDefinitionPath := getConfigPath(limitsRoot, realm, LIMITS+DEFINITION)
	limitGroup, err := LoadFile[models.LimitDefinitionGroup](limitDefinitionPath)
	if err != nil {
		return nil, err
	}

	sortNamedItems(limitGroup.Values)

	consolePropertyDefinitionPath := getConfigPath(limitsRoot, realm, CONSOLE_PROPERTIES+DEFINITION)
	consolePropertyDefinitionGroup, err := LoadFile[models.ConsolePropertyDefinitionGroup](consolePropertyDefinitionPath)
	if err != nil {
		return nil, err
	}

	sortNamedItems(consolePropertyDefinitionGroup.Values)

	propertyDefinitionPath := getConfigPath(limitsRoot, realm, PROPERTIES+DEFINITION)
	propertyDefinitionGroup, err := LoadFile[models.PropertyDefinitionGroup](propertyDefinitionPath)
	if err != nil {
		return nil, err
	}

	sortNamedItems(propertyDefinitionGroup.Values)

	tenantMap := make(map[string]tenantInfo)

	limitTenancyOverrideMap, err := loadTenancyOverrides[models.LimitTenancyOverride](
		limitsRoot, realm, LIMITS+TENANCY_OVERRIDES)
	if err != nil {
		return nil, err
	}

	updateTenants(tenantMap, limitTenancyOverrideMap, 0)

	consolePropertyTenancyOverrideMap, err := loadTenancyOverrides[models.ConsolePropertyTenancyOverride](
		limitsRoot, realm, CONSOLE_PROPERTIES+TENANCY_OVERRIDES)
	if err != nil {
		return nil, err
	}

	updateTenants(tenantMap, consolePropertyTenancyOverrideMap, 1)

	propertyTenancyOverrideMap, err := loadTenancyOverrides[models.PropertyTenancyOverride](
		limitsRoot, realm, PROPERTIES+TENANCY_OVERRIDES)
	if err != nil {
		return nil, err
	}

	updateTenants(tenantMap, propertyTenancyOverrideMap, 2)
	tenants := getTenants(tenantMap)

	consolePropertyRegionalOverrides, err := loadRegionalOverrides[models.ConsolePropertyRegionalOverride](
		limitsRoot, realm, CONSOLE_PROPERTIES+REGIONAL_OVERRIDES)
	if err != nil {
		return nil, err
	}

	propertyRegionalOverrides, err := loadRegionalOverrides[models.PropertyRegionalOverride](
		limitsRoot, realm, PROPERTIES+REGIONAL_OVERRIDES)
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
