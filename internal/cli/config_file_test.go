//nolint:paralleltest // NewRootCmd uses cobra/viper global state
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runDoctorJSON runs `toolkit --config cfgPath doctor -o json` and returns
// its rows by name, and the command's error.
func runDoctorJSON(t *testing.T, cfgPath string) (map[string]checkResult, error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)
	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"--config", cfgPath, "doctor", "-o", "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	err := cmd.Execute()
	var rows []checkResult
	require.NoError(t, json.Unmarshal(out.Bytes(), &rows), "doctor reports even when the config is broken: %s", out.String())
	byName := map[string]checkResult{}
	for _, r := range rows {
		byName[r.Name] = r
	}
	return byName, err
}

// A config file that doesn't parse is a FAIL row in doctor's report,
// not an abort before it.
func TestDoctor_MalformedConfigIsAFinding(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "cfg.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("repo-path: [unclosed\n"), 0o600))

	rows, err := runDoctorJSON(t, cfgPath)
	require.ErrorContains(t, err, "check(s) failed")
	assert.Equal(t, statusFail, rows["config_file"].Status)
	assert.Contains(t, rows["config_file"].Detail, "read config file")
	assert.Equal(t, statusSkip, rows["config_keys"].Status)
}

// A key that sets nothing (a typo) is a WARN naming it; doctor still
// passes on it.
func TestDoctor_UnknownConfigKeyWarns(t *testing.T) {
	tmp := t.TempDir()
	repo, kube := filepath.Join(tmp, "repo"), filepath.Join(tmp, "kube.yaml")
	require.NoError(t, os.Mkdir(repo, 0o750))
	require.NoError(t, os.WriteFile(kube, []byte("x"), 0o600))
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	contents := append(fullCfgContents(repo, kube), []byte("env_type: prod\nlog-format: json\n")...)
	require.NoError(t, os.WriteFile(cfgPath, contents, 0o600))

	rows, err := runDoctorJSON(t, cfgPath)
	require.NoError(t, err, "a warning doesn't fail doctor")
	assert.Equal(t, statusWarn, rows["config_keys"].Status)
	assert.Equal(t, "unknown key(s): env_type", rows["config_keys"].Detail, "log-format is a setting")
}

// A config file named with --config must exist; the default one may be
// absent.
func TestReadConfigFile_ExplicitMustExist(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)
	NewRootCmd("vtest") // sets defaultConfigFile

	missing := filepath.Join(t.TempDir(), "nope.yaml")
	require.ErrorContains(t, readConfigFile(&missing), "config file "+missing+" does not exist")

	def := defaultConfigFile
	require.NoError(t, readConfigFile(&def))
}
