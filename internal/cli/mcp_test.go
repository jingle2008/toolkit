//nolint:paralleltest // NewRootCmd uses cobra global state and the viper singleton
package cli

import (
	"strings"
	"testing"

	"github.com/jingle2008/toolkit/internal/config"
)

// TestMCPCmd_MissingRequiredSettings: `toolkit mcp` must refuse to start (and
// never block on stdio) when the loader config is incomplete, naming the
// missing flags so the operator can fix them.
func TestMCPCmd_MissingRequiredSettings(t *testing.T) {
	// Empty env: HOME at a tempdir (no config file) and the TOOLKIT_* triple
	// cleared so a fresh viper sees nothing.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TOOLKIT_REPO_PATH", "")
	t.Setenv("TOOLKIT_ENV_TYPE", "")
	t.Setenv("TOOLKIT_ENV_REGION", "")
	t.Setenv("TOOLKIT_ENV_REALM", "")

	out, err := runRootCmd(t, []string{"mcp"}, "")
	if err == nil {
		t.Fatalf("expected error when required settings are missing; output:\n%s", out)
	}
	msg := err.Error()
	if !strings.Contains(msg, "toolkit mcp") {
		t.Errorf("error should name the mcp command, got: %v", err)
	}
	if !strings.Contains(msg, "--repo-path") {
		t.Errorf("error should name the missing --repo-path flag, got: %v", err)
	}
}

// TestMCPCmd_RejectsArgs: the mcp command takes no positional args, so a stray
// arg must error rather than start the server.
func TestMCPCmd_RejectsArgs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, err := runRootCmd(t, []string{"mcp", "unexpected-arg"}, "")
	if err == nil {
		t.Fatal("expected error for unexpected positional arg to `toolkit mcp`")
	}
}

// TestMCPCmd_StartupValidatesLogFormat: with a complete loader config but an
// invalid log-format, startup must fail at logger init — before the server
// runs — proving log options are validated on the mcp path too.
func TestMCPCmd_StartupValidatesLogFormat(t *testing.T) {
	stageMutationEnv(t)                        // env triple + kubeconfig + viper.Reset
	t.Setenv("TOOLKIT_REPO_PATH", t.TempDir()) // satisfy validateLoaderConfig

	out, err := runRootCmd(t, []string{"mcp", "--log-format", "bogus"}, "")
	if err == nil {
		t.Fatalf("expected error for invalid log-format; output:\n%s", out)
	}
	if !strings.Contains(err.Error(), "invalid log-format") {
		t.Errorf("expected invalid log-format error, got: %v", err)
	}
}

// TestValidateLoaderConfig covers the shared loader-config gate used by both
// `toolkit get` and `toolkit mcp`: it must report exactly the empty required
// fields (RepoPath + env triple), and nothing when all are set.
func TestValidateLoaderConfig(t *testing.T) {
	t.Parallel()
	full := config.Config{
		RepoPath:  "/repo",
		EnvType:   "dev",
		EnvRegion: "us-ashburn-1",
		EnvRealm:  "oc1",
	}

	tests := []struct {
		name    string
		mutate  func(c *config.Config)
		missing []string
	}{
		{"complete", func(*config.Config) {}, nil},
		{"no repo", func(c *config.Config) { c.RepoPath = "" }, []string{"--repo-path"}},
		{"no env-type", func(c *config.Config) { c.EnvType = "" }, []string{"--env-type"}},
		{"no env-region", func(c *config.Config) { c.EnvRegion = "" }, []string{"--env-region"}},
		{"no env-realm", func(c *config.Config) { c.EnvRealm = "" }, []string{"--env-realm"}},
		{
			"all empty",
			func(c *config.Config) { *c = config.Config{} },
			[]string{"--repo-path", "--env-type", "--env-region", "--env-realm"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := full
			tc.mutate(&cfg)
			got := validateLoaderConfig(cfg)
			if len(tc.missing) == 0 {
				if len(got) != 0 {
					t.Fatalf("expected no missing settings, got %v", got)
				}
				return
			}
			if strings.Join(got, ",") != strings.Join(tc.missing, ",") {
				t.Errorf("missing = %v, want %v", got, tc.missing)
			}
		})
	}
}
