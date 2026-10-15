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

type idSet map[string]struct{}

func merge[T any](value T, override *T) T {
	if override == nil {
		return value
	}
	return *override
}

// getTenants builds the realm's tenants from the override directories
// (tenantMap: directory name → tenancy OCIDs) merged with tenant metadata.
// A metadata name takes precedence over the directory name, and tenants
// that end up with the same name are merged into one. It also returns the
// directory name → Tenant.Name mapping that LoadTenancyOverrideGroup uses
// to re-key the override maps, so Tenant → override scoping keeps working
// for renamed tenants. Output is deterministic: tenants are sorted by
// name, IDs are sorted, and tenants claiming the same OCID are flagged
// via SharedWith.
func getTenants(tenantMap map[string]idSet, tenantMeta []models.TenantMetadata) ([]models.Tenant, map[string]string) {
	// Build id -> TenantMetadata map for quick lookup
	idToMeta := make(map[string]models.TenantMetadata, len(tenantMeta))
	consumed := make(map[string]bool, len(tenantMeta))
	for _, m := range tenantMeta {
		idToMeta[m.ID] = m
	}

	tenants := make([]models.Tenant, 0, len(tenantMap)+len(tenantMeta))
	byName := make(map[string]int, cap(tenants))
	add := func(t models.Tenant) {
		if i, ok := byName[t.Name]; ok {
			tenants[i].IDs = append(tenants[i].IDs, t.IDs...)
			if tenants[i].Note == "" {
				tenants[i].Note = t.Note
			}
			return
		}
		byName[t.Name] = len(tenants)
		tenants = append(tenants, t)
	}

	// First: process tenantMap, merging with metadata if any id matches
	dirToName := make(map[string]string, len(tenantMap))
	for _, k := range slices.Sorted(maps.Keys(tenantMap)) {
		ids := slices.Sorted(maps.Keys(tenantMap[k]))

		tenantName := k
		isInternal := true
		note := ""
		for _, id := range ids {
			if m, ok := idToMeta[id]; ok {
				tenantName = merge(tenantName, m.Name)
				isInternal = merge(isInternal, m.IsInternal)
				note = merge(note, m.Note)
				consumed[id] = true
				break
			}
		}

		dirToName[k] = tenantName
		add(models.Tenant{
			IDs:        ids,
			Name:       tenantName,
			IsInternal: isInternal,
			Note:       note,
		})
	}

	// Second: add any metadata entries not matched above
	for _, m := range tenantMeta {
		if consumed[m.ID] || m.Name == nil || m.IsInternal == nil {
			continue
		}

		add(models.Tenant{
			IDs:        []string{m.ID},
			Name:       *m.Name,
			IsInternal: *m.IsInternal,
			Note:       merge("", m.Note),
		})
	}

	for i := range tenants {
		slices.Sort(tenants[i].IDs)
		tenants[i].IDs = slices.Compact(tenants[i].IDs)
	}
	slices.SortFunc(tenants, func(a, b models.Tenant) int {
		return strings.Compare(a.Name, b.Name)
	})
	markSharedIDs(tenants)

	return tenants, dirToName
}

// markSharedIDs sets SharedWith on every tenant that claims an OCID also
// claimed by another tenant (e.g. the same tenancy under two differently
// spelled override directories). Such tenants are flagged faulty, since
// tenant-owned rows carrying that OCID can only resolve to one of them.
func markSharedIDs(tenants []models.Tenant) {
	claimants := make(map[string][]int)
	for i, t := range tenants {
		for _, id := range t.IDs {
			claimants[id] = append(claimants[id], i)
		}
	}
	for i := range tenants {
		var shared []string
		for _, id := range tenants[i].IDs {
			for _, j := range claimants[id] {
				if j != i {
					shared = append(shared, tenants[j].Name)
				}
			}
		}
		slices.Sort(shared)
		tenants[i].SharedWith = slices.Compact(shared)
	}
}

// rekeyByTenant re-keys an override map from tenant directory name to the
// resolved Tenant.Name (see getTenants), concatenating the overrides of
// directories that resolve to the same tenant. Directories absent from
// dirToName keep their key.
func rekeyByTenant[T any](m map[string][]T, dirToName map[string]string) map[string][]T {
	out := make(map[string][]T, len(m))
	for _, dir := range slices.Sorted(maps.Keys(m)) {
		name, ok := dirToName[dir]
		if !ok {
			name = dir
		}
		out[name] = append(out[name], m[dir]...)
	}
	return out
}

// tenancyOverrideRecord is the constraint satisfied by every
// tenancy-override type loaded by loadAndStamp: NamedItem (for the
// loader path) and TenancyOverride (for the tenantMap update).
type tenancyOverrideRecord interface {
	models.NamedItem
	models.TenancyOverride
}

// loadAndStamp loads the tenancy override file set under key for the
// given realm, stamps each record with its source-directory name as
// TenantName (via setTenantName, since SetTenantName lives on the
// value pointer), and updates the running tenantMap with this
// batch's tenant IDs. The three concrete override types call this
// from LoadTenancyOverrideGroup.
func loadAndStamp[T tenancyOverrideRecord](
	ctx context.Context,
	limitsRoot, realm, key string,
	tenantMap map[string]idSet,
	setTenantName func(*T, string),
) (map[string][]T, error) {
	m, err := loadTenancyOverrides[T](ctx, limitsRoot, realm, key)
	if err != nil {
		return nil, err
	}
	for name, ovs := range m {
		for i := range ovs {
			setTenantName(&ovs[i], name)
		}
	}
	updateTenants(tenantMap, m)
	return m, nil
}

func updateTenants[T models.TenancyOverride](
	tenantMap map[string]idSet, overrideMap map[string][]T,
) {
	for name, overrides := range overrideMap {
		info, ok := tenantMap[name]
		if !ok {
			info = idSet{}
			tenantMap[name] = info
		}

		for _, o := range overrides {
			tenantID := o.GetTenantID()
			info[tenantID] = struct{}{}
		}
	}
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
	tenantMap := make(map[string]idSet)
	limitsRoot := getLimitsRoot(repoPath)

	limitTenancyOverrideMap, err := loadAndStamp(
		ctx, limitsRoot, realm, limitsKey+tenancyOverridesKey, tenantMap,
		func(v *models.LimitTenancyOverride, name string) { v.SetTenantName(name) },
	)
	if err != nil {
		return models.TenancyOverrideGroup{}, err
	}

	consolePropertyTenancyOverrideMap, err := loadAndStamp(
		ctx, limitsRoot, realm, consolePropertiesKey+tenancyOverridesKey, tenantMap,
		func(v *models.ConsolePropertyTenancyOverride, name string) { v.SetTenantName(name) },
	)
	if err != nil {
		return models.TenancyOverrideGroup{}, err
	}

	propertyTenancyOverrideMap, err := loadAndStamp(
		ctx, limitsRoot, realm, propertiesKey+tenancyOverridesKey, tenantMap,
		func(v *models.PropertyTenancyOverride, name string) { v.SetTenantName(name) },
	)
	if err != nil {
		return models.TenancyOverrideGroup{}, err
	}

	tenants, dirToName := getTenants(tenantMap, metadata.GetTenants(realm))
	return models.TenancyOverrideGroup{
		Tenants:                           tenants,
		LimitTenancyOverrideMap:           rekeyByTenant(limitTenancyOverrideMap, dirToName),
		ConsolePropertyTenancyOverrideMap: rekeyByTenant(consolePropertyTenancyOverrideMap, dirToName),
		PropertyTenancyOverrideMap:        rekeyByTenant(propertyTenancyOverrideMap, dirToName),
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
