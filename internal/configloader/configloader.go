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
	"strings"

	"slices"

	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	fs "github.com/jingle2008/toolkit/internal/fileutil"
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

func getConfigPath(root, configName string) string {
	const realm = "oc1"
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
	ctx context.Context,
	dirPath string,
	listFilesFunc func(context.Context, string, string) ([]string, error),
	loadFileFunc func(string) (*T, error),
) ([]T, error) {
	overrideFiles, err := listFilesFunc(ctx, dirPath, ".json")
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

	slices.SortFunc(overrides, func(a, b T) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	return overrides, nil
}

func loadOverrides[T models.NamedItem](ctx context.Context, dirPath string) ([]T, error) {
	return loadOverridesDI(ctx, dirPath, fs.ListFiles, jsonutil.LoadFile[T])
}

func loadTenancyOverridesDI[T models.NamedItem](
	ctx context.Context,
	root, realm, name string,
	listSubDirsFunc func(string) ([]string, error),
	loadOverridesFunc func(context.Context, string) ([]T, error),
) (map[string][]T, error) {
	results := make(map[string][]T)

	realmDir := filepath.Join(root, name, regionalValuesDir, realm)
	tenants, err := listSubDirsFunc(realmDir)
	if err != nil {
		return nil, err
	}

	for _, tenant := range tenants {
		overrides, err := loadOverridesFunc(ctx, filepath.Join(realmDir, tenant))
		if err != nil {
			return nil, err
		}
		results[tenant] = overrides
	}

	return results, nil
}

func loadTenancyOverrides[T models.NamedItem](ctx context.Context, root, realm, name string) (map[string][]T, error) {
	return loadTenancyOverridesDI(ctx, root, realm, name, listSubDirs, loadOverrides[T])
}

func loadRegionalOverrides[T models.NamedItem](ctx context.Context, root, realm, name string) ([]T, error) {
	realmDir := filepath.Join(root, name, regionalValuesDir, realm)
	overrides, err := loadOverrides[T](ctx, realmDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []T{}, nil
		}
		return nil, err
	}
	return overrides, nil
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

	slices.SortFunc(tenants, func(a, b models.Tenant) int {
		return strings.Compare(a.Name, b.Name)
	})
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

	slices.SortFunc(environments, func(a, b models.Environment) int {
		return strings.Compare(a.GetKey(), b.GetKey())
	})
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

	realm := env.Realm

	limitGroup, consolePropertyDefinitionGroup, propertyDefinitionGroup, err := loadDefinitionGroups(repoPath)
	if err != nil {
		return nil, err
	}

	tenancyOverrideGroup, err := LoadTenancyOverrideGroup(ctx, repoPath, realm)
	if err != nil {
		return nil, err
	}

	limitRegionalOverrides, err := LoadLimitRegionalOverrides(ctx, repoPath, realm)
	if err != nil {
		return nil, err
	}
	consolePropertyRegionalOverrides, err := LoadConsolePropertyRegionalOverrides(ctx, repoPath, realm)
	if err != nil {
		return nil, err
	}
	propertyRegionalOverrides, err := LoadPropertyRegionalOverrides(ctx, repoPath, realm)
	if err != nil {
		return nil, err
	}

	modelArtifactMap, err := terraform.LoadModelArtifacts(ctx, repoPath, env)
	if err != nil {
		return nil, err
	}

	return &models.Dataset{
		LimitDefinitionGroup:              *limitGroup,
		ConsolePropertyDefinitionGroup:    *consolePropertyDefinitionGroup,
		PropertyDefinitionGroup:           *propertyDefinitionGroup,
		LimitTenancyOverrideMap:           tenancyOverrideGroup.LimitTenancyOverrideMap,
		ConsolePropertyTenancyOverrideMap: tenancyOverrideGroup.ConsolePropertyTenancyOverrideMap,
		PropertyTenancyOverrideMap:        tenancyOverrideGroup.PropertyTenancyOverrideMap,
		ConsolePropertyRegionalOverrides:  consolePropertyRegionalOverrides,
		LimitRegionalOverrides:            limitRegionalOverrides,
		PropertyRegionalOverrides:         propertyRegionalOverrides,
		Tenants:                           tenancyOverrideGroup.Tenants,
		Environments:                      environments,
		ServiceTenancies:                  serviceTenancies,
		ModelArtifactMap:                  modelArtifactMap,
	}, nil
}

func getLimitsRoot(repoPath string) string {
	return filepath.Join(repoPath, "shared_modules/limits")
}

// validateEnvironment checks if the provided environment is valid.
func validateEnvironment(env models.Environment, allEnvs []models.Environment) error {
	if !isValidEnvironment(env, allEnvs) {
		return errors.New("environment is not valid or in the list")
	}
	return nil
}

// loadDefinitionGroups loads and sorts the definition groups.
// NOTE: Definitions are always loaded from the "oc1" realm, regardless of the user's selected realm.
func loadDefinitionGroups(repoPath string) (
	*models.LimitDefinitionGroup,
	*models.ConsolePropertyDefinitionGroup,
	*models.PropertyDefinitionGroup,
	error,
) {
	limitsRoot := getLimitsRoot(repoPath)
	limitDefinitionPath := getConfigPath(limitsRoot, limitsKey+definitionSuffix)
	limitGroup, err := jsonutil.LoadFile[models.LimitDefinitionGroup](limitDefinitionPath)
	if err != nil {
		return nil, nil, nil, err
	}
	slices.SortFunc(limitGroup.Values, func(a, b models.LimitDefinition) int {
		return strings.Compare(a.Name, b.Name)
	})

	consolePropertyDefinitionPath := getConfigPath(limitsRoot, consolePropertiesKey+definitionSuffix)
	consolePropertyDefinitionGroup, err := jsonutil.LoadFile[models.ConsolePropertyDefinitionGroup](consolePropertyDefinitionPath)
	if err != nil {
		return nil, nil, nil, err
	}
	slices.SortFunc(consolePropertyDefinitionGroup.Values, func(a, b models.ConsolePropertyDefinition) int {
		return strings.Compare(a.Name, b.Name)
	})

	propertyDefinitionPath := getConfigPath(limitsRoot, propertiesKey+definitionSuffix)
	propertyDefinitionGroup, err := jsonutil.LoadFile[models.PropertyDefinitionGroup](propertyDefinitionPath)
	if err != nil {
		return nil, nil, nil, err
	}
	slices.SortFunc(propertyDefinitionGroup.Values, func(a, b models.PropertyDefinition) int {
		return strings.Compare(a.Name, b.Name)
	})

	return limitGroup, consolePropertyDefinitionGroup, propertyDefinitionGroup, nil
}

/*
LoadTenancyOverrideGroup loads tenants and all tenancy override maps for a given realm.
*/
func LoadTenancyOverrideGroup(ctx context.Context, repoPath, realm string) (models.TenancyOverrideGroup, error) {
	tenantMap := make(map[string]tenantInfo)
	limitsRoot := getLimitsRoot(repoPath)

	limitTenancyOverrideMap, err := loadTenancyOverrides[models.LimitTenancyOverride](
		ctx, limitsRoot, realm, limitsKey+tenancyOverridesKey)
	if err != nil {
		return models.TenancyOverrideGroup{}, err
	}
	updateTenants(tenantMap, limitTenancyOverrideMap, 0)

	consolePropertyTenancyOverrideMap, err := loadTenancyOverrides[models.ConsolePropertyTenancyOverride](
		ctx, limitsRoot, realm, consolePropertiesKey+tenancyOverridesKey)
	if err != nil {
		return models.TenancyOverrideGroup{}, err
	}
	updateTenants(tenantMap, consolePropertyTenancyOverrideMap, 1)

	propertyTenancyOverrideMap, err := loadTenancyOverrides[models.PropertyTenancyOverride](
		ctx, limitsRoot, realm, propertiesKey+tenancyOverridesKey)
	if err != nil {
		return models.TenancyOverrideGroup{}, err
	}
	updateTenants(tenantMap, propertyTenancyOverrideMap, 2)

	tenants := getTenants(tenantMap)
	return models.TenancyOverrideGroup{
		Tenants:                           tenants,
		LimitTenancyOverrideMap:           limitTenancyOverrideMap,
		ConsolePropertyTenancyOverrideMap: consolePropertyTenancyOverrideMap,
		PropertyTenancyOverrideMap:        propertyTenancyOverrideMap,
	}, nil
}

// LoadLimitRegionalOverrides loads limit regional overrides for the given repo path and realm.
func LoadLimitRegionalOverrides(ctx context.Context, repoPath, realm string) ([]models.LimitRegionalOverride, error) {
	return loadRegionalOverrides[models.LimitRegionalOverride](ctx, getLimitsRoot(repoPath), realm, limitsKey+regionalOverridesKey)
}

// LoadConsolePropertyRegionalOverrides loads console property regional overrides for the given repo path and realm.
func LoadConsolePropertyRegionalOverrides(ctx context.Context, repoPath, realm string) ([]models.ConsolePropertyRegionalOverride, error) {
	return loadRegionalOverrides[models.ConsolePropertyRegionalOverride](ctx, getLimitsRoot(repoPath), realm, consolePropertiesKey+regionalOverridesKey)
}

// LoadPropertyRegionalOverrides loads property regional overrides for the given repo path and realm.
func LoadPropertyRegionalOverrides(ctx context.Context, repoPath, realm string) ([]models.PropertyRegionalOverride, error) {
	return loadRegionalOverrides[models.PropertyRegionalOverride](ctx, getLimitsRoot(repoPath), realm, propertiesKey+regionalOverridesKey)
}
