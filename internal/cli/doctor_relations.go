package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/internal/relations"
	"github.com/jingle2008/toolkit/pkg/models"
)

// doctorLoaderFn builds the loader `doctor --relations` reads through; a
// test seam.
var doctorLoaderFn = production.New

// relationExamples caps the dangling rows the table detail names; -o
// json/yaml list every one under items.
const relationExamples = 3

// relationChecks runs the relation integrity checks (`doctor --relations`):
// it loads every category for the configured environment, resolves tenant
// owners, and reports one row per relation, plus the tenancy check. When
// the config can't reach the loaders it reports a single SKIP row; when a
// load fails, a single FAIL row.
func relationChecks(ctx context.Context, cfg config.Config) []checkResult {
	missing := validateLoaderConfig(cfg)
	if cfg.KubeConfig == "" {
		missing = append(missing, "--kubeconfig")
	}
	if len(missing) > 0 {
		return []checkResult{{
			Name: "relations", Status: statusSkip,
			Detail: "missing " + strings.Join(missing, ", "),
			Hint:   "set them via flags, environment (TOOLKIT_*), or config.yaml",
		}}
	}
	env := models.Environment{Type: cfg.EnvType, Region: cfg.EnvRegion, Realm: cfg.EnvRealm}
	ds, poolNote, err := loadIntegrityDataset(ctx, doctorLoaderFn(ctx, cfg.MetadataFile), cfg, env)
	if err != nil {
		return []checkResult{{
			Name: "relations", Status: statusFail, Detail: err.Error(),
			Hint: "check cluster access (kubeconfig, OCI session) and repo-path",
		}}
	}
	checks := relations.CheckIntegrity(ds)
	out := make([]checkResult, 0, len(checks))
	for _, c := range checks {
		if poolNote != "" && c.Child == domain.GPUNode && c.Parent == domain.GPUPool && len(c.Dangling) > 0 {
			// Still FAIL, as the TUI flags these nodes faulty: a missing
			// pool may just be unloaded, but a broken source needs fixing
			// too, and the note names it.
			c.Note = joinNonEmpty(c.Note, poolNote)
		}
		out = append(out, relationResult(c))
	}
	return out
}

// loadIntegrityDataset loads every category the relations touch,
// concurrently, and resolves tenant owners as the TUI does. A partial
// GPU-pool load is not fatal: poolNote describes it.
func loadIntegrityDataset(ctx context.Context, ld loader.Composite, cfg config.Config, env models.Environment) (ds *models.Dataset, poolNote string, err error) {
	var (
		bms    []models.BaseModel
		srs    []models.ServingRuntime
		ims    map[string][]models.ImportedModel
		pools  []models.GPUPool
		nodes  map[string][]models.GPUNode
		pods   map[string][]models.GPUWorkload
		dacs   map[string][]models.DedicatedAICluster
		isvcs  map[string][]models.InferenceService
		repoDS *models.Dataset
	)
	g, gctx := errgroup.WithContext(ctx)
	load := func(what string, fn func() error) {
		g.Go(func() error {
			if err := fn(); err != nil {
				return fmt.Errorf("load %s: %w", what, err)
			}
			return nil
		})
	}
	load("dataset", func() (err error) { repoDS, err = ld.LoadDataset(gctx, cfg.RepoPath, env); return })
	load("base models", func() (err error) { bms, err = ld.LoadBaseModels(gctx, cfg.KubeConfig, env); return })
	load("serving runtimes", func() (err error) { srs, err = ld.LoadServingRuntimes(gctx, cfg.KubeConfig, env); return })
	load("imported models", func() (err error) { ims, err = ld.LoadImportedModels(gctx, cfg.KubeConfig, env); return })
	load("gpu nodes", func() (err error) { nodes, err = ld.LoadGPUNodesByPool(gctx, cfg.KubeConfig, env); return })
	load("gpu workloads", func() (err error) { pods, err = ld.LoadGPUWorkloadsByNode(gctx, cfg.KubeConfig, env); return })
	load("dedicated AI clusters", func() (err error) { dacs, err = ld.LoadDedicatedAIClusters(gctx, cfg.KubeConfig, env); return })
	load("inference services", func() (err error) { isvcs, err = ld.LoadInferenceServices(gctx, cfg.KubeConfig, env); return })
	load("gpu pools", func() error {
		var err error
		pools, err = ld.LoadGPUPools(gctx, cfg.RepoPath, env)
		if partial, ok := errors.AsType[*terraform.PartialLoadError](err); ok {
			poolNote = "some GPU pool sources failed to load: " + partial.Error()
			return nil
		}
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, "", err
	}

	ds = repoDS
	ds.BaseModels, ds.ServingRuntimes, ds.GPUPools, ds.GPUNodeMap = bms, srs, pools, nodes
	ds.SetDedicatedAIClusterMap(dacs)
	ds.SetImportedModelMap(ims)
	ds.SetGPUWorkloadMap(pods)
	ds.SetInferenceServiceMap(isvcs)
	return ds, poolNote, nil
}

// relationResult renders one integrity check as a doctor row.
func relationResult(c relations.Check) checkResult {
	r := checkResult{Name: "relations: " + c.Name, Status: statusPass}
	detail := fmt.Sprintf("%d checked", c.Checked)
	if n := len(c.Dangling); n > 0 {
		r.Status = checkStatus(c.Severity)
		examples := make([]string, 0, min(n, relationExamples))
		for _, d := range c.Dangling[:min(n, relationExamples)] {
			examples = append(examples, fmt.Sprintf("%s (missing %s)", d.Row, d.Missing))
		}
		if n > relationExamples {
			examples = append(examples, fmt.Sprintf("+%d more", n-relationExamples))
		}
		detail += fmt.Sprintf("; %d dangling: %s", n, strings.Join(examples, ", "))
		for _, d := range c.Dangling {
			r.Items = append(r.Items, d.Row+" → "+d.Missing)
		}
		r.Hint = relationHint(c)
	}
	r.Detail = joinNonEmpty(detail, c.Note)
	return r
}

func relationHint(c relations.Check) string {
	if c.Parent == domain.Tenant {
		return "add the tenancies to the metadata file (TUI: edit tenant on an unresolved DAC/ImportedModel)"
	}
	return fmt.Sprintf("these %s rows name a %s that doesn't exist: clean them up or restore the %s", c.Child, c.Parent, c.Parent)
}

func joinNonEmpty(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "; ")
}
