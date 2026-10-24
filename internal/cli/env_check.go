package cli

import (
	"context"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// loadEnvironmentsFn is the seam tests use for the repo's environments.
var loadEnvironmentsFn = configloader.LoadEnvironments

// checkGetEnvironment is checkEnvironment for `get <cat>`: Alias is a
// static list that loads nothing, so any env does.
func checkGetEnvironment(ctx context.Context, cfg config.Config, cat domain.Category) error {
	if cat == domain.Alias {
		return nil
	}
	return checkEnvironment(ctx, cfg)
}

// checkEnvironment refuses an env triple the repo doesn't declare, as
// the TUI's dataset load does. Without it a mistyped realm (or a region
// of another realm) failed open: the cluster loaders key on region and
// type only, so get, metrics and the mutations acted on the configured
// cluster under a realm that doesn't exist, with tenants unresolved and
// OCIDs naming nothing. When the repo's environments can't be loaded,
// nothing is refused: the command's own loaders report the repo's state.
func checkEnvironment(ctx context.Context, cfg config.Config) error {
	if cfg.RepoPath == "" || cfg.EnvType == "" || cfg.EnvRegion == "" || cfg.EnvRealm == "" {
		return nil
	}
	all, err := loadEnvironmentsFn(ctx, cfg.RepoPath)
	if err != nil {
		logging.FromContext(ctx).Warn("environment not checked: repo environments didn't load", "error", err)
		return nil
	}
	return configloader.CheckEnvironment(models.Environment{Type: cfg.EnvType, Region: cfg.EnvRegion, Realm: cfg.EnvRealm}, all)
}
