package cli

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/output"
)

// addDoctorCommand wires `toolkit doctor`, a read-only health check
// that aggregates the file-existence and schema checks scattered
// across the subcommands (validateGetConfig, runRootE, …) into one
// place so operators can confirm their setup before running anything.
func addDoctorCommand(rootCmd *cobra.Command, cfgFile *string) {
	var (
		format    string
		relations bool
	)
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run health checks against the current configuration",
		Long: `Run health checks against the current toolkit configuration.

doctor inspects what get/mcp/<mutation> commands would see at startup —
the merged config, the config file, repo-path, kubeconfig, metadata-file
— and reports each check as PASS / FAIL / SKIP with a short remediation
hint when something is wrong.

--relations also checks the data itself: it loads every category for the
configured environment and, for each relation the TUI drills through,
reports rows that name a parent that doesn't exist — an endpoint whose
DAC was deleted, a pod naming a missing runtime, a node in a pool absent
from terraform. These FAIL. Rows whose tenancy resolves to no known
tenant are reported as WARN: a gap in the tenant metadata, not broken
state. -o json/yaml lists every dangling row under items.

Exit code is non-zero if any check fails, so doctor fits into
precondition scripts:

  if ! toolkit doctor; then
    echo "fix the failures above" >&2
    exit 1
  fi

Without --relations, doctor never makes a network call — it is purely
a local file + schema audit. (Cluster reachability + OCI credential
checks may be added under a future --connectivity flag.)

Examples:
  toolkit doctor
  toolkit doctor -o json
  toolkit doctor -o yaml
  toolkit doctor --relations
  toolkit doctor --relations -o json | jq '.[] | select(.status != "PASS")'`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			// A config file that doesn't read is a finding, not a reason to
			// stop: doctor reports it with the other checks.
			readErr := readConfigFile(cfgFile)
			// Ctrl+C cancels the cluster loads of --relations (c.Context()
			// is Execute's signal context).
			return runDoctor(c.Context(), c.OutOrStdout(), *cfgFile, format, relations,
				configFileRead{err: readErr, knownKeys: settingKeys(c.Root())})
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "table", "table|json|yaml")
	cmd.Flags().BoolVar(&relations, "relations", false,
		"also check the data's cross-category references (loads every category from the cluster and repo)")
	_ = cmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "json", "yaml"}, cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(cmd)
}

// checkStatus is the verdict of a single doctor check.
type checkStatus string

const (
	statusPass checkStatus = "PASS"
	statusFail checkStatus = "FAIL"
	statusSkip checkStatus = "SKIP"
	// statusWarn flags a gap that isn't broken state (an unresolved
	// tenancy); it doesn't fail doctor.
	statusWarn checkStatus = "WARN"
)

// checkResult is a single row of the doctor report. Detail is the
// human-readable observation; Hint is the actionable next step
// (empty when the check passed).
type checkResult struct {
	Name   string      `json:"name"`
	Status checkStatus `json:"status"`
	Detail string      `json:"detail,omitempty"`
	Hint   string      `json:"hint,omitempty"`
	// Items lists every offending row (doctor --relations), where Detail
	// names only the first few.
	Items []string `json:"items,omitempty"`
}

// configFileRead is what doctor knows about reading the config file:
// the error reading it, if any, and the keys a config file may set.
type configFileRead struct {
	err       error
	knownKeys map[string]bool
}

// settingKeys is every key a config file may set: the root command's
// persistent flags, which viper binds by name (bar --config itself).
func settingKeys(root *cobra.Command) map[string]bool {
	keys := map[string]bool{}
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name != "config" {
			keys[f.Name] = true
		}
	})
	return keys
}

func runDoctor(ctx context.Context, w io.Writer, cfgFile, format string, relations bool, read configFileRead) error {
	// Validate format before doing any work — a typo'd -o shouldn't
	// stat the filesystem (and stays consistent with `config`).
	f, err := output.ParseFormatIn(format, output.FormatTable, output.FormatTable, output.FormatJSON, output.FormatYAML)
	if err != nil {
		return err
	}

	var cfg config.Config
	unmarshalErr := viper.Unmarshal(&cfg)
	// Doctor reports rather than aborts, so an unresolvable region code
	// rides the same channel as a schema error: checkConfigSchema is
	// already "the config loads and is usable", which this is part of.
	// cfg keeps its raw values on failure, which is what the remaining
	// path checks want to show anyway.
	if unmarshalErr == nil {
		unmarshalErr = cfg.Normalize()
	}
	results := collectChecks(cfgFile, cfg, unmarshalErr, read)
	if relations {
		if countFails(results) > 0 {
			results = append(results, checkResult{
				Name: "relations", Status: statusSkip,
				Detail: "skipped: the configuration checks above failed",
				Hint:   "fix those first",
			})
		} else {
			results = append(results, relationChecks(ctx, cfg)...)
		}
	}

	switch f {
	case output.FormatJSON:
		err = output.WriteJSON(w, results, output.Options{Pretty: true})
	case output.FormatYAML:
		err = output.WriteYAML(w, results, output.Options{Pretty: true})
	default:
		err = writeDoctorTable(w, results)
	}
	if err != nil {
		return err
	}

	if n := countFails(results); n > 0 {
		return fmt.Errorf("doctor: %d check(s) failed", n)
	}
	return nil
}

// collectChecks runs each individual probe and returns the rows. The
// list order is stable so output diff-tests cleanly across runs.
func collectChecks(cfgFile string, cfg config.Config, unmarshalErr error, read configFileRead) []checkResult {
	file := checkConfigFile(cfgFile)
	if file.Status == statusPass && read.err != nil {
		file = checkResult{
			Name: "config_file", Status: statusFail, Detail: read.err.Error(),
			Hint: "fix the file's YAML/JSON syntax",
		}
	}
	return []checkResult{
		checkConfigSchema(cfg, unmarshalErr),
		file,
		checkConfigKeys(cfgFile, file.Status == statusPass, read.knownKeys),
		checkPath("repo-path", cfg.RepoPath, true, "set --repo-path or `repo-path:` in config.yaml"),
		// kubeconfig is `required=true` to match config.Validate(): the
		// TUI insists on it and the cluster-derived categories (BaseModel,
		// GPUNode, DAC) refuse to load without it. If you only ever use
		// repo-derived categories from `toolkit get` and want a SKIP
		// instead of a FAIL, that's a future cfg.ValidateFor(category)
		// feature — until then doctor must not contradict Validate.
		checkPath("kubeconfig", cfg.KubeConfig, true, "set --kubeconfig or run `kind/minikube/oke` setup"),
		checkMetadataFile(cfg.MetadataFile),
	}
}

func checkConfigSchema(cfg config.Config, unmarshalErr error) checkResult {
	if unmarshalErr != nil {
		return checkResult{
			Name:   "config_schema",
			Status: statusFail,
			Detail: unmarshalErr.Error(),
			Hint:   "fix the config syntax; `toolkit config` shows the merged view",
		}
	}
	if err := cfg.Validate(); err != nil {
		return checkResult{
			Name:   "config_schema",
			Status: statusFail,
			Detail: err.Error(),
			Hint:   "run `toolkit init` to scaffold a complete config",
		}
	}
	return checkResult{Name: "config_schema", Status: statusPass}
}

func checkConfigFile(cfgFile string) checkResult {
	r := checkResult{Name: "config_file"}
	if cfgFile == "" {
		r.Status = statusSkip
		r.Detail = "(--config '' disables file load)"
		return r
	}
	switch _, err := os.Stat(cfgFile); {
	case err == nil:
		r.Status = statusPass
		r.Detail = cfgFile
	case os.IsNotExist(err):
		r.Status = statusFail
		r.Detail = cfgFile + " does not exist"
		r.Hint = "run `toolkit init` to scaffold ~/.config/toolkit/config.yaml"
	default:
		r.Status = statusFail
		r.Detail = err.Error()
	}
	return r
}

// checkConfigKeys warns about keys in the config file that set nothing:
// a misspelled one (env-typo, env_type) is otherwise silently ignored, and
// the setting it meant stays at its default. It is skipped when the file
// didn't read (readOK false) or known is empty.
func checkConfigKeys(cfgFile string, readOK bool, known map[string]bool) checkResult {
	r := checkResult{Name: "config_keys"}
	if !readOK || len(known) == 0 {
		r.Status, r.Detail = statusSkip, "no config file read"
		return r
	}
	v := viper.New()
	v.SetConfigFile(cfgFile)
	if err := v.ReadInConfig(); err != nil {
		r.Status, r.Detail = statusSkip, err.Error()
		return r
	}
	unknown := map[string]bool{}
	for _, k := range v.AllKeys() {
		if top, _, _ := strings.Cut(k, "."); !known[top] {
			unknown[top] = true
		}
	}
	if len(unknown) == 0 {
		r.Status = statusPass
		return r
	}
	r.Status = statusWarn
	r.Detail = "unknown key(s): " + strings.Join(slices.Sorted(maps.Keys(unknown)), ", ")
	r.Hint = "check the spelling against `toolkit --help` (the flag names); unknown keys are ignored"
	return r
}

// checkPath verifies value points at an existing path (file or
// directory). When required is false an empty value SKIPs rather
// than FAILing — for kubeconfig, which is optional for repo-only
// categories.
func checkPath(name, value string, required bool, hint string) checkResult {
	r := checkResult{Name: name}
	if value == "" {
		if required {
			r.Status = statusFail
			r.Detail = "not set"
			r.Hint = hint
		} else {
			r.Status = statusSkip
			r.Detail = "not set"
		}
		return r
	}
	if _, err := os.Stat(value); err != nil {
		r.Status = statusFail
		r.Detail = err.Error()
		r.Hint = hint
		return r
	}
	r.Status = statusPass
	r.Detail = value
	return r
}

// checkMetadataFile is path-shaped but always optional, with its own
// remediation hint pointing back to the example config.
func checkMetadataFile(path string) checkResult {
	r := checkResult{Name: "metadata-file"}
	if path == "" {
		r.Status = statusSkip
		r.Detail = "not set"
		return r
	}
	if _, err := os.Stat(path); err != nil {
		// Default value points at ~/.config/toolkit/metadata.yaml which
		// most users won't have. Treat "does not exist" as SKIP so the
		// happy path doesn't FAIL on every fresh install, but FAIL on
		// other stat errors (permission denied, etc.) since those
		// indicate a real misconfiguration.
		if os.IsNotExist(err) {
			r.Status = statusSkip
			r.Detail = path + " not present (optional)"
			return r
		}
		r.Status = statusFail
		r.Detail = err.Error()
		r.Hint = "fix permissions or unset metadata-file"
		return r
	}
	r.Status = statusPass
	r.Detail = path
	return r
}

func countFails(results []checkResult) int {
	n := 0
	for _, r := range results {
		if r.Status == statusFail {
			n++
		}
	}
	return n
}

func writeDoctorTable(w io.Writer, results []checkResult) error {
	headers := []string{"CHECK", "STATUS", "DETAIL", "HINT"}
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		rows = append(rows, []string{r.Name, string(r.Status), r.Detail, r.Hint})
	}
	// Propagate non-pipe errors. A broken pipe (the consumer hung up)
	// is fine to surface as well — the cobra error path just sets exit
	// non-zero, which is what the caller expects in that case.
	return output.WriteTable(w, headers, rows, output.Options{})
}
