package resolve

import (
	"context"
	"fmt"

	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/models"
)

// TenantDataset loads env's tenants into an otherwise empty Dataset so
// callers can run the Dataset.Set*Map resolvers — the same tenant-owner
// resolution the TUI applies on load (DAC / ImportedModel maps re-keyed
// by Tenant.Name, Owner set on every tenant-owned item). The returned
// Dataset is never nil: on a load error it carries no tenants, so the
// resolvers keep the raw tenancy-id keys and callers can degrade to
// unresolved output with a warning.
func TenantDataset(ctx context.Context, ld loader.TenancyOverrideLoader, repoPath string, env models.Environment) (*models.Dataset, error) {
	grp, err := ld.LoadTenancyOverrideGroup(ctx, repoPath, env)
	if err != nil {
		return &models.Dataset{}, fmt.Errorf("load tenants: %w", err)
	}
	return &models.Dataset{Tenants: grp.Tenants}, nil
}
