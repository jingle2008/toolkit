//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// stageDACEnv is the minimal env for DAC operations: env triple only,
// NO kubeconfig (DAC delete is OCI-only).
func stageDACEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TOOLKIT_ENV_TYPE", "dev")
	t.Setenv("TOOLKIT_ENV_REGION", "us-ashburn-1")
	t.Setenv("TOOLKIT_ENV_REALM", "oc1")
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestDeleteDAC_DryRun_DoesNotCallOCI(t *testing.T) {
	stageDACEnv(t)
	called := false
	defer swap(&deleteDACFn, func(context.Context, *models.DedicatedAICluster, models.Environment, logging.Logger) error {
		called = true
		return nil
	})()

	out, err := runRootCmd(t, []string{"delete", "dac", "dac-x", "--dry-run"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not call OCI")
	}
	if !strings.Contains(out, "DRY-RUN: would delete dac/dac-x") {
		t.Errorf("expected DRY-RUN line, got: %q", out)
	}
}

func TestDeleteDAC_RequiresExplicitYes(t *testing.T) {
	stageDACEnv(t)
	called := false
	defer swap(&deleteDACFn, func(context.Context, *models.DedicatedAICluster, models.Environment, logging.Logger) error {
		called = true
		return nil
	})()

	_, err := runRootCmd(t, []string{"delete", "dac", "dac-x"}, "y\n") // typing y must NOT be enough
	if err == nil {
		t.Fatal("expected error: destructive op requires --yes")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error should mention --yes: %v", err)
	}
	if called {
		t.Fatal("must not call OCI without --yes")
	}
}

func TestDeleteDAC_YesCallsOCI(t *testing.T) {
	stageDACEnv(t)
	var gotDAC *models.DedicatedAICluster
	defer swap(&deleteDACFn, func(_ context.Context, d *models.DedicatedAICluster, _ models.Environment, _ logging.Logger) error {
		gotDAC = d
		return nil
	})()

	out, err := runRootCmd(t, []string{"delete", "dac", "dac-x", "--yes"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if gotDAC == nil || gotDAC.Name != "dac-x" {
		t.Errorf("expected DAC with Name=dac-x, got: %+v", gotDAC)
	}
	if !strings.Contains(out, "delete dac/dac-x: OK") {
		t.Errorf("expected OK, got: %q", out)
	}
}

func TestDeleteDAC_PerformError(t *testing.T) {
	stageDACEnv(t)
	defer swap(&deleteDACFn, func(context.Context, *models.DedicatedAICluster, models.Environment, logging.Logger) error {
		return errors.New("work request FAILED")
	})()

	_, err := runRootCmd(t, []string{"delete", "dac", "dac-x", "--yes"}, "")
	if err == nil {
		t.Fatal("expected error to surface")
	}
	if !strings.Contains(err.Error(), "work request FAILED") {
		t.Errorf("error must wrap underlying: %v", err)
	}
}
