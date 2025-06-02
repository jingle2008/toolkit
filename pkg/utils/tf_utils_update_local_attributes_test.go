package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
)

func writeTempFile(t *testing.T, dir, name, content string) string {
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0o644)
	assert.NoError(t, err)
	return path
}

func TestUpdateLocalAttributes_LocalsBlock(t *testing.T) {
	dir := t.TempDir()
	tf := `
locals {
  foo = "bar"
  num = 42
}
`
	path := writeTempFile(t, dir, "locals.tf", tf)
	attrs := make(hclsyntax.Attributes)
	err := updateLocalAttributes(path, attrs)
	assert.NoError(t, err)
	assert.Contains(t, attrs, "foo")
	assert.Contains(t, attrs, "num")
}

func TestUpdateLocalAttributes_OutputBlock(t *testing.T) {
	dir := t.TempDir()
	tf := `
output "baz" {
  value = "qux"
}
`
	path := writeTempFile(t, dir, "output.tf", tf)
	attrs := make(hclsyntax.Attributes)
	err := updateLocalAttributes(path, attrs)
	assert.NoError(t, err)
	assert.Contains(t, attrs, "baz")
}

func TestUpdateLocalAttributes_NoRelevantBlocks(t *testing.T) {
	dir := t.TempDir()
	tf := `
resource "null_resource" "test" {}
`
	path := writeTempFile(t, dir, "none.tf", tf)
	attrs := make(hclsyntax.Attributes)
	err := updateLocalAttributes(path, attrs)
	assert.NoError(t, err)
	assert.Len(t, attrs, 0)
}

func TestUpdateLocalAttributes_InvalidHCL(t *testing.T) {
	dir := t.TempDir()
	tf := `
locals {
  foo = 
`
	path := writeTempFile(t, dir, "bad.tf", tf)
	attrs := make(hclsyntax.Attributes)
	err := updateLocalAttributes(path, attrs)
	assert.Error(t, err)
}
