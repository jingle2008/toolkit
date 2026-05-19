package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/internal/config"
)

// addDoctorCommand wires `toolkit doctor`, a read-only health check
// that aggregates the file-existence and schema checks scattered
// across the subcommands (validateGetConfig, runRootE, …) into one
// place so operators can confirm their setup before running anything.
func addDoctorCommand(rootCmd *cobra.Command, cfgFile *string) {
	var format string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run health checks against the current configuration",
		Long: `Run health checks against the current toolkit configuration.

doctor inspects what get/mcp/<mutation> commands would see at startup —
the merged config, the config file, repo_path, kubeconfig, metadata_file
— and reports each check as PASS / FAIL / SKIP with a short remediation
hint when something is wrong.

Exit code is non-zero if any check fails, so doctor fits into
precondition scripts:

  if ! toolkit doctor; then
    echo "fix the failures above" >&2
    exit 1
  fi

doctor never makes a network call — it is purely a local file +
schema audit. (Cluster reachability + OCI credential checks may be
added under a future --connectivity flag.)

Examples:
  toolkit doctor
  toolkit doctor -o json
  toolkit doctor -o yaml`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := readConfigFile(cfgFile); err != nil {
				return err
			}
			return runDoctor(c.OutOrStdout(), *cfgFile, format)
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "table", "table|json|yaml")
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
)

// checkResult is a single row of the doctor report. Detail is the
// human-readable observation; Hint is the actionable next step
// (empty when the check passed).
type checkResult struct {
	Name   string      `json:"name" yaml:"name"`
	Status checkStatus `json:"status" yaml:"status"`
	Detail string      `json:"detail,omitempty" yaml:"detail,omitempty"`
	Hint   string      `json:"hint,omitempty" yaml:"hint,omitempty"`
}

func runDoctor(w io.Writer, cfgFile, format string) error {
	var cfg config.Config
	unmarshalErr := viper.Unmarshal(&cfg)

	results := collectChecks(cfgFile, cfg, unmarshalErr)

	switch strings.ToLower(format) {
	case "", "table":
		writeDoctorTable(w, results)
	case "json":
		if err := output.WriteJSON(w, results, output.Options{Pretty: true}); err != nil {
			return err
		}
	case "yaml":
		if err := output.WriteYAML(w, results, output.Options{Pretty: true}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid output format %q (valid: table|json|yaml)", format)
	}

	for _, r := range results {
		if r.Status == statusFail {
			return fmt.Errorf("doctor: %d check(s) failed", countFails(results))
		}
	}
	return nil
}

// collectChecks runs each individual probe and returns the rows. The
// list order is stable so output diff-tests cleanly across runs.
func collectChecks(cfgFile string, cfg config.Config, unmarshalErr error) []checkResult {
	return []checkResult{
		checkConfigSchema(cfg, unmarshalErr),
		checkConfigFile(cfgFile),
		checkPath("repo_path", cfg.RepoPath, true, "set --repo_path or `repo_path:` in config.yaml"),
		checkPath("kubeconfig", cfg.KubeConfig, false, "set --kubeconfig or run `kind/minikube/oke` setup"),
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
	r := checkResult{Name: "metadata_file"}
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
		r.Hint = "fix permissions or unset metadata_file"
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

func writeDoctorTable(w io.Writer, results []checkResult) {
	headers := []string{"CHECK", "STATUS", "DETAIL", "HINT"}
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		rows = append(rows, []string{r.Name, string(r.Status), r.Detail, r.Hint})
	}
	// Errors from WriteTable can only happen on writer failure, which
	// for stdout means the consumer hung up. Nothing we can do but
	// swallow it — the deferred error path inside RunE is already
	// where doctor surfaces non-zero exit.
	_ = output.WriteTable(w, headers, rows, output.Options{})
}
