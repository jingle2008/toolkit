package tui

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// lazySource is how a category that loads on its own — not with the
// repo's dataset at startup — loads, stores and watches its rows. A new
// such category needs only its entry in lazySources.
type lazySource struct {
	// withDataset marks rows the repo's dataset load brings too: the
	// category loads alone only when they're missing, and revisiting it
	// doesn't reload them. The others load when first shown, and reload
	// on every visit.
	withDataset bool
	// loaded reports whether ds holds the category's rows.
	loaded func(ds *models.Dataset) bool
	// load fetches the rows, off the Update loop.
	load func(ctx context.Context, in loadInput) (loadedRows, error)
	// watch, when set, starts the category's cluster watch.
	watch func(ctx context.Context, w loader.Watcher, kubeConfig string, env models.Environment) (<-chan struct{}, error)
	// applied, when set, follows the rows being stored (a GPU pool's
	// state comes from OCI).
	applied func(m *Model) tea.Cmd
}

// loadInput is what a load reads, snapshotted from the model when the
// command is built: the command reads nothing of the model.
type loadInput struct {
	ld         loader.Composite
	repoPath   string
	kubeConfig string
	env        models.Environment
	// withDACs loads the DACs beside GPU workloads, for the tenants of
	// pods without a tenancy-id label, when none are loaded.
	withDACs bool
}

// loadedRows is a load's result: apply stores it in a dataset, n counts
// its rows.
type loadedRows struct {
	apply func(ds *models.Dataset)
	n     int
}

// categoryLoadedMsg carries a lazy category's loaded rows.
type categoryLoadedMsg struct {
	Cat  domain.Category
	Gen  int
	Rows loadedRows
}

var lazySources = map[domain.Category]lazySource{
	domain.BaseModel: {
		loaded: func(ds *models.Dataset) bool { return ds.BaseModels != nil },
		load: func(ctx context.Context, in loadInput) (loadedRows, error) {
			items, err := in.ld.LoadBaseModels(ctx, in.kubeConfig, in.env)
			return baseModelRows(items), err
		},
		watch: func(ctx context.Context, w loader.Watcher, kc string, env models.Environment) (<-chan struct{}, error) {
			return w.WatchBaseModels(ctx, kc, env)
		},
	},
	domain.ServingRuntime: {
		loaded: func(ds *models.Dataset) bool { return ds.ServingRuntimes != nil },
		load: func(ctx context.Context, in loadInput) (loadedRows, error) {
			items, err := in.ld.LoadServingRuntimes(ctx, in.kubeConfig, in.env)
			return servingRuntimeRows(items), err
		},
		watch: func(ctx context.Context, w loader.Watcher, kc string, env models.Environment) (<-chan struct{}, error) {
			return w.WatchServingRuntimes(ctx, kc, env)
		},
	},
	domain.ImportedModel: {
		loaded: func(ds *models.Dataset) bool { return ds.ImportedModelMap != nil },
		load: func(ctx context.Context, in loadInput) (loadedRows, error) {
			items, err := in.ld.LoadImportedModels(ctx, in.kubeConfig, in.env)
			return importedModelRows(items), err
		},
		watch: func(ctx context.Context, w loader.Watcher, kc string, env models.Environment) (<-chan struct{}, error) {
			return w.WatchImportedModels(ctx, kc, env)
		},
	},
	domain.GPUPool: {
		loaded: func(ds *models.Dataset) bool { return ds.GPUPools != nil },
		load: func(ctx context.Context, in loadInput) (loadedRows, error) {
			items, err := in.ld.LoadGPUPools(ctx, in.repoPath, in.env)
			// A partial load is non-fatal in the TUI: items has the rows
			// that loaded, and the terraform package logged each failure.
			if partial, ok := errors.AsType[*terraform.PartialLoadError](err); ok {
				logging.FromContext(ctx).Warn("loaded GPU pools with partial failures",
					"category", domain.GPUPool, "error", partial)
				err = nil
			}
			return gpuPoolRows(items), err
		},
		applied: func(m *Model) tea.Cmd { return m.updateGPUPoolState() },
	},
	domain.GPUNode: {
		loaded: func(ds *models.Dataset) bool { return ds.GPUNodeMap != nil },
		load: func(ctx context.Context, in loadInput) (loadedRows, error) {
			items, err := in.ld.LoadGPUNodesByPool(ctx, in.kubeConfig, in.env)
			return gpuNodeRows(items), err
		},
		watch: func(ctx context.Context, w loader.Watcher, kc string, env models.Environment) (<-chan struct{}, error) {
			return w.WatchGPUNodes(ctx, kc, env)
		},
	},
	domain.GPUWorkload: {
		loaded: func(ds *models.Dataset) bool { return ds.GPUWorkloadMap != nil },
		load:   loadGPUWorkloads,
		watch: func(ctx context.Context, w loader.Watcher, kc string, env models.Environment) (<-chan struct{}, error) {
			return w.WatchGPUWorkloads(ctx, kc, env)
		},
	},
	domain.DedicatedAICluster: {
		loaded: func(ds *models.Dataset) bool { return ds.DedicatedAIClusterMap != nil },
		load: func(ctx context.Context, in loadInput) (loadedRows, error) {
			items, err := in.ld.LoadDedicatedAIClusters(ctx, in.kubeConfig, in.env)
			return dacRows(items), err
		},
		watch: func(ctx context.Context, w loader.Watcher, kc string, env models.Environment) (<-chan struct{}, error) {
			return w.WatchDedicatedAIClusters(ctx, kc, env)
		},
	},
	domain.InferenceService: {
		loaded: func(ds *models.Dataset) bool { return ds.InferenceServiceMap != nil },
		load: func(ctx context.Context, in loadInput) (loadedRows, error) {
			items, err := in.ld.LoadInferenceServices(ctx, in.kubeConfig, in.env)
			return inferenceServiceRows(items), err
		},
		watch: func(ctx context.Context, w loader.Watcher, kc string, env models.Environment) (<-chan struct{}, error) {
			return w.WatchInferenceServices(ctx, kc, env)
		},
	},
	domain.LimitRegionalOverride: {
		withDataset: true,
		loaded:      func(ds *models.Dataset) bool { return ds.LimitRegionalOverrides != nil },
		load: func(ctx context.Context, in loadInput) (loadedRows, error) {
			items, err := in.ld.LoadLimitRegionalOverrides(ctx, in.repoPath, in.env)
			return limitRegionalOverrideRows(items), err
		},
	},
	domain.ConsolePropertyRegionalOverride: {
		withDataset: true,
		loaded:      func(ds *models.Dataset) bool { return ds.ConsolePropertyRegionalOverrides != nil },
		load: func(ctx context.Context, in loadInput) (loadedRows, error) {
			items, err := in.ld.LoadConsolePropertyRegionalOverrides(ctx, in.repoPath, in.env)
			return consolePropertyRegionalOverrideRows(items), err
		},
	},
	domain.PropertyRegionalOverride: {
		withDataset: true,
		loaded:      func(ds *models.Dataset) bool { return ds.PropertyRegionalOverrides != nil },
		load: func(ctx context.Context, in loadInput) (loadedRows, error) {
			items, err := in.ld.LoadPropertyRegionalOverrides(ctx, in.repoPath, in.env)
			return propertyRegionalOverrideRows(items), err
		},
	},
}

// loadGPUWorkloads loads the GPU workloads and, withDACs, the DACs too,
// at the same time. A failed DAC load only leaves pods without a
// tenancy-id label unowned.
func loadGPUWorkloads(ctx context.Context, in loadInput) (loadedRows, error) {
	var dacs chan map[string][]models.DedicatedAICluster
	if in.withDACs {
		dacs = make(chan map[string][]models.DedicatedAICluster, 1)
		go func() {
			m, _ := in.ld.LoadDedicatedAIClusters(ctx, in.kubeConfig, in.env)
			dacs <- m
		}()
	}
	items, err := in.ld.LoadGPUWorkloadsByNode(ctx, in.kubeConfig, in.env)
	if err != nil {
		return loadedRows{}, err
	}
	var loadedDACs map[string][]models.DedicatedAICluster
	if in.withDACs {
		loadedDACs = <-dacs
	}
	return gpuWorkloadRows(items, loadedDACs), nil
}

// The rows of each lazy category, as its load stores them.

func baseModelRows(items []models.BaseModel) loadedRows {
	return loadedRows{func(ds *models.Dataset) { ds.BaseModels = items }, len(items)}
}

func servingRuntimeRows(items []models.ServingRuntime) loadedRows {
	return loadedRows{func(ds *models.Dataset) { ds.ServingRuntimes = items }, len(items)}
}

func importedModelRows(items map[string][]models.ImportedModel) loadedRows {
	return loadedRows{func(ds *models.Dataset) { ds.SetImportedModelMap(items) }, mapLen(items)}
}

func gpuPoolRows(items []models.GPUPool) loadedRows {
	return loadedRows{func(ds *models.Dataset) { ds.GPUPools = items }, len(items)}
}

func gpuNodeRows(items map[string][]models.GPUNode) loadedRows {
	return loadedRows{func(ds *models.Dataset) { ds.GPUNodeMap = items }, mapLen(items)}
}

// gpuWorkloadRows stores the workloads, and the DACs loaded with them
// when none are loaded yet (a DAC load of its own may have landed
// meanwhile, and is as fresh).
func gpuWorkloadRows(items map[string][]models.GPUWorkload, dacs map[string][]models.DedicatedAICluster) loadedRows {
	return loadedRows{func(ds *models.Dataset) {
		if dacs != nil && ds.DedicatedAIClusterMap == nil {
			setDedicatedAIClusters(ds, dacs)
		}
		ds.SetGPUWorkloadMap(items)
	}, mapLen(items)}
}

func dacRows(items map[string][]models.DedicatedAICluster) loadedRows {
	return loadedRows{func(ds *models.Dataset) { setDedicatedAIClusters(ds, items) }, mapLen(items)}
}

func inferenceServiceRows(items map[string][]models.InferenceService) loadedRows {
	return loadedRows{func(ds *models.Dataset) { ds.SetInferenceServiceMap(items) }, mapLen(items)}
}

func limitRegionalOverrideRows(items []models.LimitRegionalOverride) loadedRows {
	return loadedRows{func(ds *models.Dataset) { ds.LimitRegionalOverrides = items }, len(items)}
}

func consolePropertyRegionalOverrideRows(items []models.ConsolePropertyRegionalOverride) loadedRows {
	return loadedRows{func(ds *models.Dataset) { ds.ConsolePropertyRegionalOverrides = items }, len(items)}
}

func propertyRegionalOverrideRows(items []models.PropertyRegionalOverride) loadedRows {
	return loadedRows{func(ds *models.Dataset) { ds.PropertyRegionalOverrides = items }, len(items)}
}

// loadInput snapshots what a load reads.
func (m *Model) loadInput() loadInput {
	return loadInput{
		ld:         m.loader,
		repoPath:   m.repoPath,
		kubeConfig: m.kubeConfig,
		env:        m.environment,
		withDACs:   m.workloadsNeedDACs(),
	}
}

// loadCategoryCmd loads lazy category cat, its result tagged gen so a
// stale one is dropped; nil for any other category.
func (m *Model) loadCategoryCmd(cat domain.Category, gen int) tea.Cmd {
	src, ok := lazySources[cat]
	if !ok {
		return nil
	}
	ctx, in := m.loadCtx, m.loadInput()
	return func() tea.Msg {
		rows, err := src.load(ctx, in)
		if err != nil {
			return errMsg{err: fmt.Errorf("failed to load %s: %w", cat, err), Gen: gen, Cat: cat}
		}
		return categoryLoadedMsg{Cat: cat, Gen: gen, Rows: rows}
	}
}

// categoryLoad is the load navigating to cat makes, if any: when its rows
// are missing, or on a refresh of a category that reloads on revisit.
func (m *Model) categoryLoad(cat domain.Category, src lazySource, refresh bool, gen int) tea.Cmd {
	if m.dataset == nil || !src.loaded(m.dataset) || (refresh && !src.withDataset) {
		return m.loadCategoryCmd(cat, gen)
	}
	return nil
}

// handleCategoryLoaded stores a lazy category's rows, unless stale.
func (m *Model) handleCategoryLoaded(msg categoryLoadedMsg) tea.Cmd {
	if !m.applyLoaded(msg.Gen, msg.Rows.apply, msg.Cat, msg.Rows.n) {
		return nil
	}
	if applied := lazySources[msg.Cat].applied; applied != nil {
		return applied(m)
	}
	return nil
}
