package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/relations"
	"github.com/jingle2008/toolkit/internal/resolve"
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
	ds, poolNote, err := resolve.IntegrityDataset(ctx, doctorLoaderFn(ctx, cfg.MetadataFile), cfg.RepoPath, cfg.KubeConfig, env)
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
