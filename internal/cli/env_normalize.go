package cli

import (
	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
)

/*
logEnvAliases records any env alias that Config.Normalize expanded.

Every downstream consumer sees only the canonical value, which is what
OCIDs, kubecontexts, and OCI SDK endpoints are built from. That's the
right default, but it means a wrong-looking OCID in a later log line
gives no hint that a short code was ever involved. One line at startup
closes that gap without polluting the rest of the log.

Silent when nothing was expanded, so the common case adds nothing.
*/
func logEnvAliases(logger logging.Logger, rawType, rawRegion string, cfg config.Config) {
	if rawType != cfg.EnvType {
		logger.Infow("resolved env-type alias", "from", rawType, "to", cfg.EnvType)
	}
	if rawRegion != cfg.EnvRegion {
		logger.Infow("resolved env-region code", "from", rawRegion, "to", cfg.EnvRegion)
	}
}
