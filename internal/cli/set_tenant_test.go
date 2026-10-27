//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/pkg/models"
)

const testTenancyOCID = "ocid1.tenancy.oc1..aaaaexample"

// stageTenantEnv sets only HOME — NO env triple — proving set-tenant
// does not require --env-type/region/realm.
func stageTenantEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestSetTenant_DryRun_DoesNotWrite(t *testing.T) {
	d := &ops.Deps{}
	stageTenantEnv(t)
	called := false
	d.UpsertTenant = func(context.Context, models.TenantMetadata) error {
		called = true
		return nil
	}

	out, err := runRootCmdWith(t, d, []string{"set", "tenant", testTenancyOCID, "--name", "Acme", "--dry-run"}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("--dry-run must not write")
	}
	if !strings.Contains(out, "DRY-RUN: would set tenant/"+testTenancyOCID) {
		t.Errorf("expected DRY-RUN line, got: %q", out)
	}
}

func TestSetTenant_RequiresName(t *testing.T) {
	d := &ops.Deps{}
	stageTenantEnv(t)
	d.UpsertTenant = func(context.Context, models.TenantMetadata) error {
		t.Fatal("must not write when --name missing")
		return nil
	}

	_, err := runRootCmdWith(t, d, []string{"set", "tenant", testTenancyOCID, "--yes"}, "")
	if err == nil {
		t.Fatal("expected error when --name missing")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention name: %v", err)
	}
}

func TestSetTenant_RejectsNonTenancyOCID(t *testing.T) {
	d := &ops.Deps{}
	stageTenantEnv(t)
	d.UpsertTenant = func(context.Context, models.TenantMetadata) error {
		t.Fatal("must not write for bad OCID")
		return nil
	}

	_, err := runRootCmdWith(t, d, []string{"set", "tenant", "not-an-ocid", "--name", "Acme", "--yes"}, "")
	if err == nil {
		t.Fatal("expected error for non-tenancy OCID")
	}
	if !strings.Contains(err.Error(), "ocid1.tenancy.") {
		t.Errorf("error should mention the expected prefix: %v", err)
	}
}

func TestSetTenant_YesWritesEntry(t *testing.T) {
	d := &ops.Deps{}
	stageTenantEnv(t)
	var got models.TenantMetadata
	d.UpsertTenant = func(_ context.Context, e models.TenantMetadata) error {
		got = e
		return nil
	}

	out, err := runRootCmdWith(t, d, []string{
		"set", "tenant", testTenancyOCID,
		"--name", "Acme", "--internal=false", "--note", "vip", "--yes",
	}, "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got.ID != testTenancyOCID {
		t.Errorf("ID: want %q, got %q", testTenancyOCID, got.ID)
	}
	if got.Name == nil || *got.Name != "Acme" {
		t.Errorf("Name: want Acme, got %v", got.Name)
	}
	if got.IsInternal == nil || *got.IsInternal != false {
		t.Errorf("IsInternal: want false, got %v", got.IsInternal)
	}
	if got.Note == nil || *got.Note != "vip" {
		t.Errorf("Note: want vip, got %v", got.Note)
	}
	if !strings.Contains(out, "set tenant/"+testTenancyOCID+": OK") {
		t.Errorf("expected OK, got: %q", out)
	}
}

func TestSetTenant_InternalAndNoteOmittedWhenUnset(t *testing.T) {
	d := &ops.Deps{}
	stageTenantEnv(t)
	var got models.TenantMetadata
	d.UpsertTenant = func(_ context.Context, e models.TenantMetadata) error {
		got = e
		return nil
	}

	if _, err := runRootCmdWith(t, d, []string{"set", "tenant", testTenancyOCID, "--name", "Acme", "--yes"}, ""); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// Unset, so the upsert keeps the recorded flag (new tenants read as internal).
	if got.IsInternal != nil {
		t.Errorf("IsInternal should be nil without --internal, got %v", *got.IsInternal)
	}
	if got.Note != nil {
		t.Errorf("Note should be nil when --note empty, got %v", *got.Note)
	}
}

func TestSetTenant_PerformError(t *testing.T) {
	d := &ops.Deps{}
	stageTenantEnv(t)
	d.UpsertTenant = func(context.Context, models.TenantMetadata) error {
		return errors.New("disk full")
	}

	_, err := runRootCmdWith(t, d, []string{"set", "tenant", testTenancyOCID, "--name", "Acme", "--yes"}, "")
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("expected wrapped perform error, got: %v", err)
	}
}
