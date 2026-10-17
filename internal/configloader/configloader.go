/*
Package configloader provides utilities for loading configuration overrides and tenancy data for the toolkit.
*/
package configloader

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	"github.com/jingle2008/toolkit/internal/fileutil"
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

func loadOverridesWith[T models.NamedItem](
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

	return overrides, nil
}

func loadOverrides[T models.NamedItem](ctx context.Context, dirPath string) ([]T, error) {
	return loadOverridesWith(ctx, dirPath, fileutil.ListFiles, jsonutil.LoadFile[T])
}

func loadTenancyOverridesWith[T models.NamedItem](
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
	return loadTenancyOverridesWith(ctx, root, realm, name, listSubDirs, loadOverrides[T])
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

func merge[T any](value T, override *T) T {
	if override == nil {
		return value
	}
	return *override
}

// getTenants builds the realm's tenants: one per tenancy OCID, taken from
// the override records (ids) and the realm's metadata entries. Metadata
// is the only source of names; an OCID without a metadata name is named
// by the OCID itself, so names stay one to one with IDs (metadata names
// are validated unique on load). A tenancy not marked otherwise in
// metadata is internal. Sorted by name.
func getTenants(ids map[string]bool, tenantMeta []models.TenantMetadata) []models.Tenant {
	byID := make(map[string]models.Tenant, len(ids)+len(tenantMeta))
	for id := range ids {
		byID[id] = models.Tenant{ID: id, Name: id, IsInternal: true}
	}
	for _, m := range tenantMeta {
		t := models.Tenant{ID: m.ID, Name: m.ID, IsInternal: merge(true, m.IsInternal), Note: merge("", m.Note), Status: m.Status}
		if m.Name != nil && *m.Name != "" {
			t.Name = *m.Name
		}
		byID[m.ID] = t
	}
	tenants := slices.Collect(maps.Values(byID))
	slices.SortFunc(tenants, func(a, b models.Tenant) int {
		return strings.Compare(a.Name, b.Name)
	})
	return tenants
}

// collectTenantIDs adds every override record's tenancy OCID to ids.
func collectTenantIDs[T models.TenancyOverride](ids map[string]bool, byDir map[string][]T) {
	for _, overrides := range byDir {
		for _, o := range overrides {
			if id := o.GetTenantID(); id != "" {
				ids[id] = true
			}
		}
	}
}

// groupByTenant regroups overrides, loaded per tenant directory, by the
// tenant each record names through its own tenancy OCID, and stamps that
// tenant's name on the record. Directory names play no part: they aren't
// reliable tenant identities.
func groupByTenant[T models.TenancyOverride](byDir map[string][]T, nameOf map[string]string, setTenantName func(*T, string)) map[string][]T {
	out := make(map[string][]T)
	for _, dir := range slices.Sorted(maps.Keys(byDir)) {
		for _, o := range byDir[dir] {
			name := nameOf[o.GetTenantID()]
			setTenantName(&o, name)
			out[name] = append(out[name], o)
		}
	}
	return out
}

func getEnvironments(tenancies []models.ServiceTenancy) []models.Environment {
	var environments []models.Environment
	for _, t := range tenancies {
		environments = append(environments, t.Environments()...)
	}

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
func LoadDataset(ctx context.Context, repoPath string, env models.Environment, metadata *models.Metadata) (*models.Dataset, error) {
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

	tenancyOverrideGroup, err := LoadTenancyOverrideGroup(ctx, repoPath, realm, metadata)
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

// splatLimitDefinitionsFile holds the splat throttle limit definitions,
// a bare JSON array that the limits_definitions Terraform module appends
// to oc1_limits_definition.json's group as one more group
// (all_limits_definitions). Regional and tenancy overrides reference
// them by name like any other limit.
const splatLimitDefinitionsFile = "splat_limit_definitions.json"

// loadSplatLimitDefinitions loads the splat throttle definitions, or none
// when the file is absent (repo revisions predating it).
func loadSplatLimitDefinitions(limitsRoot string) ([]models.LimitDefinition, error) {
	path := filepath.Join(limitsRoot, limitsKey+definitionSuffix+"s", splatLimitDefinitionsFile)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	defs, err := jsonutil.LoadFile[[]models.LimitDefinition](path)
	if err != nil {
		return nil, err
	}
	return *defs, nil
}

// loadDefinitionGroups loads the definition groups.
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
	splat, err := loadSplatLimitDefinitions(limitsRoot)
	if err != nil {
		return nil, nil, nil, err
	}
	limitGroup.Values = append(limitGroup.Values, splat...)

	consolePropertyDefinitionPath := getConfigPath(limitsRoot, consolePropertiesKey+definitionSuffix)
	consolePropertyDefinitionGroup, err := jsonutil.LoadFile[models.ConsolePropertyDefinitionGroup](consolePropertyDefinitionPath)
	if err != nil {
		return nil, nil, nil, err
	}

	propertyDefinitionPath := getConfigPath(limitsRoot, propertiesKey+definitionSuffix)
	propertyDefinitionGroup, err := jsonutil.LoadFile[models.PropertyDefinitionGroup](propertyDefinitionPath)
	if err != nil {
		return nil, nil, nil, err
	}

	return limitGroup, consolePropertyDefinitionGroup, propertyDefinitionGroup, nil
}

/*
LoadTenancyOverrideGroup loads tenants and all tenancy override maps for a given realm.
*/
func LoadTenancyOverrideGroup(ctx context.Context, repoPath, realm string, metadata *models.Metadata) (models.TenancyOverrideGroup, error) {
	limitsRoot := getLimitsRoot(repoPath)
	limits, err := loadTenancyOverrides[models.LimitTenancyOverride](ctx, limitsRoot, realm, limitsKey+tenancyOverridesKey)
	if err != nil {
		return models.TenancyOverrideGroup{}, err
	}
	consoleProperties, err := loadTenancyOverrides[models.ConsolePropertyTenancyOverride](ctx, limitsRoot, realm, consolePropertiesKey+tenancyOverridesKey)
	if err != nil {
		return models.TenancyOverrideGroup{}, err
	}
	properties, err := loadTenancyOverrides[models.PropertyTenancyOverride](ctx, limitsRoot, realm, propertiesKey+tenancyOverridesKey)
	if err != nil {
		return models.TenancyOverrideGroup{}, err
	}

	ids := make(map[string]bool)
	collectTenantIDs(ids, limits)
	collectTenantIDs(ids, consoleProperties)
	collectTenantIDs(ids, properties)
	tenants := getTenants(ids, metadata.GetTenants(realm))
	nameOf := make(map[string]string, len(tenants))
	for _, t := range tenants {
		nameOf[t.ID] = t.Name
	}
	return models.TenancyOverrideGroup{
		Tenants:                           tenants,
		LimitTenancyOverrideMap:           groupByTenant(limits, nameOf, (*models.LimitTenancyOverride).SetTenantName),
		ConsolePropertyTenancyOverrideMap: groupByTenant(consoleProperties, nameOf, (*models.ConsolePropertyTenancyOverride).SetTenantName),
		PropertyTenancyOverrideMap:        groupByTenant(properties, nameOf, (*models.PropertyTenancyOverride).SetTenantName),
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
