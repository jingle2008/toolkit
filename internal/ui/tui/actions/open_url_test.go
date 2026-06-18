package actions

import (
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenURL_LaunchesPlatformOpener(t *testing.T) {
	t.Parallel()

	var gotName string
	var gotArgs []string
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })
	execCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = args
		// "true" is a harmless no-op present on darwin/linux CI runners;
		// .Start() on it succeeds without opening anything.
		return exec.Command("true")
	}

	const url = "https://devops.oci.oraclecorp.com/account/admin/detail/metadata/ocid1.tenancy.oc1..abc?realm=oc1"
	require.NoError(t, OpenURL(url))

	want := map[string]string{
		"darwin":  "open",
		"windows": "rundll32",
	}[runtime.GOOS]
	if want == "" {
		want = "xdg-open"
	}
	assert.Equal(t, want, gotName)
	// The URL is always the final argument regardless of platform.
	require.NotEmpty(t, gotArgs)
	assert.Equal(t, url, gotArgs[len(gotArgs)-1])
}

func TestOpenURL_StartErrorIsWrapped(t *testing.T) {
	t.Parallel()

	orig := execCommand
	t.Cleanup(func() { execCommand = orig })
	execCommand = func(_ string, _ ...string) *exec.Cmd {
		// A command that cannot start: empty path → Start returns an error.
		return exec.Command("")
	}

	err := OpenURL("https://example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "https://example.com")
}
