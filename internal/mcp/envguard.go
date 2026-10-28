package mcp

import (
	"context"
	"encoding/json"
	"sync"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/pkg/models"
)

// mcpLoadEnvironmentsFn is the seam tests use for the repo's environment
// list, which envGuard checks overrides against.
var mcpLoadEnvironmentsFn = configloader.LoadEnvironments

// mutationTools are the tools whose env overrides take effect only with
// --mutation-env-override-allowed (effectiveMutationEnv); without it
// they're ignored, so envGuard doesn't check them either.
// TestMutationTools_ListedForEnvGuard keeps the set in step with the
// registered tools.
var mutationTools = map[string]bool{
	"cordon_node": true, "uncordon_node": true, "drain_node": true,
	"restart_node": true, "terminate_node": true, "scale_gpu_pool": true,
	"delete_dac": true, "restart_inference_service": true,
	"delete_inference_service": true, "set_tenant": true,
}

// envCache holds the repo's environments, loaded on first use and
// reloaded when an override isn't in them. mu guards the fields, not the
// load. Loads run concurrently, so each takes a ticket when it starts
// and stores its result only if no later-started load has stored first.
type envCache struct {
	mu      sync.Mutex
	envs    []models.Environment
	tickets uint64 // loads started
	stored  uint64 // ticket of the load envs came from
}

// envGuard refuses a tool call whose env_type/env_region/env_realm
// override names an environment the repo doesn't declare, before its
// handler runs. Without it a bad override failed open: the cluster
// loaders key on region and type only, so a mistyped realm loaded the
// startup cluster's rows under a realm that doesn't exist (and built
// OCIDs from it), a bad region code returned an empty list as success,
// and the repo loaders read nothing for a realm in the wrong case.
func (s *Server) envGuard(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
		call, ok := req.(*sdk.CallToolRequest)
		if method != "tools/call" || !ok || call.Params == nil {
			return next(ctx, method, req)
		}
		if mutationTools[call.Params.Name] && !s.cfg.MutationEnvOverrideAllowed {
			return next(ctx, method, req)
		}
		var in envOverride
		if json.Unmarshal(call.Params.Arguments, &in) != nil || in == (envOverride{}) {
			// Malformed arguments are the handler's (the SDK's schema
			// check) to refuse; no override leaves the startup env.
			return next(ctx, method, req)
		}
		if err := s.checkEnvOverride(ctx, in); err != nil {
			res := &sdk.CallToolResult{}
			res.SetError(err)
			return res, nil
		}
		return next(ctx, method, req)
	}
}

// checkEnvOverride reports why in names no environment the repo
// declares, or nil. A region code that resolves to nothing says so. When
// the repo's environments can't be loaded the call isn't refused: its
// loaders report the repo's state themselves.
func (s *Server) checkEnvOverride(ctx context.Context, in envOverride) error {
	if in.EnvRegion != "" {
		if _, err := models.ResolveRegion(in.EnvRegion); err != nil {
			return err
		}
	}
	env := s.envFor(in)
	s.envCache.mu.Lock()
	cached := s.envCache.envs
	if cached != nil && configloader.CheckEnvironment(env, cached) == nil {
		s.envCache.mu.Unlock()
		return nil
	}
	s.envCache.tickets++
	ticket := s.envCache.tickets
	s.envCache.mu.Unlock()
	// Not cached yet, or not in the cached list: (re)load, since the repo
	// may have gained the environment since. The load reads the repo, so
	// it runs unlocked: other calls' overrides are checked meanwhile
	// against the cached list.
	envs, err := mcpLoadEnvironmentsFn(ctx, s.cfg.RepoPath)
	if err != nil {
		s.logger.Warn("env override not checked: repo environments didn't load", "error", err)
		return nil
	}
	s.envCache.mu.Lock()
	if ticket > s.envCache.stored {
		s.envCache.envs, s.envCache.stored = envs, ticket
	}
	s.envCache.mu.Unlock()
	return configloader.CheckEnvironment(env, envs)
}
