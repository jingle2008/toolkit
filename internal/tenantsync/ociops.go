package tenantsync

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

// OCIOps fetches account details with the oci-ops CLI.
type OCIOps struct {
	Region string
}

// runOCIOpsFn runs oci-ops with args; a seam for tests.
var runOCIOpsFn = func(ctx context.Context, args ...string) (stdout, stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, "oci-ops", args...) //nolint:gosec // fixed subcommand; only the tenancy OCID varies
	// The version check refuses to run an older CLI; it doesn't affect
	// the call. Stdin stays closed so a re-authenticate prompt fails the
	// call instead of waiting for input.
	cmd.Env = append(os.Environ(), "OCI_OPS_DISABLE_VERSION_CHECK=True")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err = cmd.Run()
	return out.Bytes(), errOut.Bytes(), err
}

// Fetch implements Fetcher.
func (o OCIOps) Fetch(ctx context.Context, ocid string) ([]byte, error) {
	stdout, stderr, err := runOCIOpsFn(ctx,
		"account-admin-tool", "accounts", "get-account-details",
		"--region", o.Region, "--account", ocid)
	if err == nil {
		if _, perr := ParseAccount(stdout); perr == nil {
			return stdout, nil
		}
	}
	return nil, classify(stdout, stderr, err)
}
