package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jingle2008/toolkit/internal/infra/terraform"
)

func TestTerraformLocalAttributes_HappyPath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	tfPath := filepath.Join(tmp, "locals.tf")
	content := `
locals {
  foo = "bar"
}
`
	if err := os.WriteFile(tfPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write tf: %v", err)
	}
	attrs, err := terraform.GetLocalAttributes(context.Background(), tmp)
	if err != nil {
		t.Fatalf("getLocalAttributes failed: %v", err)
	}
	if _, ok := attrs["foo"]; !ok {
		t.Errorf("expected foo in attributes, got: %v", attrs)
	}
}
