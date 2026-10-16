package production

import (
	"sync"

	"github.com/jingle2008/toolkit/pkg/models"
)

// tenancyCache memoizes LoadTenancyOverrideGroup per repo and realm.
// Parsing every override file costs ~350ms on oc1, and the MCP server's
// tenant-owned list tools load the group on every call just for its
// tenants. An entry is reused only while the override files'
// fingerprint and the metadata it was built from are unchanged;
// UpsertTenantMetadata replaces the metadata pointer, so a metadata write
// invalidates it too. A nil cache never hits.
type tenancyCache struct {
	mu      sync.Mutex
	entries map[tenancyCacheKey]tenancyCacheEntry
}

type tenancyCacheKey struct{ repo, realm string }

type tenancyCacheEntry struct {
	fingerprint string
	metadata    *models.Metadata
	group       models.TenancyOverrideGroup
}

func newTenancyCache() *tenancyCache {
	return &tenancyCache{entries: make(map[tenancyCacheKey]tenancyCacheEntry)}
}

func (c *tenancyCache) get(key tenancyCacheKey, fingerprint string, metadata *models.Metadata) (models.TenancyOverrideGroup, bool) {
	if c == nil {
		return models.TenancyOverrideGroup{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || e.fingerprint != fingerprint || e.metadata != metadata {
		return models.TenancyOverrideGroup{}, false
	}
	return e.group, true
}

func (c *tenancyCache) put(key tenancyCacheKey, fingerprint string, metadata *models.Metadata, group models.TenancyOverrideGroup) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = tenancyCacheEntry{fingerprint: fingerprint, metadata: metadata, group: group}
}
