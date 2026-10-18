package terraform

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/jingle2008/toolkit/pkg/models"
)

func mkdir(t *testing.T, parts ...string) string {
	t.Helper()
	dir := filepath.Join(parts...)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	return dir
}

// The oc1 targets moved into a child module (genai-shepherd-flocks
// #3665): shep_targets reads module.oc1_inventory.oc1_prod, and the
// module exports each local under its own name.
func TestLoadServiceTenancies_ModuleCall(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeTfFile(t, mkdir(t, repo, "shared_modules", "shep_targets"), "target.tf", `
module "oc1_inventory" {
  source = "../oc1_region_inventory"
}

locals {
  oc1_prod = module.oc1_inventory.oc1_prod
  oc2 = {
    tenancy_name = "genaiprodoc2"
    home_region  = "us-langley-1"
    regions      = ["us-langley-1"]
    environment  = "prod"
  }
}
`)
	writeTfFile(t, mkdir(t, repo, "shared_modules", "oc1_region_inventory"), "main.tf", `
variable "connections" {
  default = { "oc1:generativeaiprod:prod:us-ashburn-1" = "x", "oc1:other:dev:us-phoenix-1" = "y" }
}

locals {
  oc1_prod = {
    tenancy_name = "generativeaiprod"
    home_region  = "us-ashburn-1"
    regions      = ["us-chicago-1", "us-ashburn-1"]
    environment  = "prod"
    enrolled = toset([
      for source, _ in var.connections : split(":", source)[3]
      if startswith(source, "oc1:generativeaiprod:prod:")
    ])
  }
}

output "oc1_prod" {
  value = local.oc1_prod
}
`)
	tenancies, err := LoadServiceTenancies(context.Background(), repo)
	require.NoError(t, err)
	byRealm := map[string]models.ServiceTenancy{}
	for _, tn := range tenancies {
		byRealm[tn.Realm] = tn
	}
	require.Contains(t, byRealm, "oc1")
	assert.Equal(t, "generativeaiprod", byRealm["oc1"].Name)
	assert.Equal(t, []string{"us-ashburn-1", "us-chicago-1"}, byRealm["oc1"].Regions)
	assert.Contains(t, byRealm, "oc2")
}

// A module call's inputs override the module's variable defaults, nested
// calls resolve, and a non-local source is skipped.
func TestModuleOutputs_InputsNestingAndRemote(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	parent := mkdir(t, root, "parent")
	writeTfFile(t, parent, "main.tf", `
variable "env" {
  default = "prod"
}

module "child" {
  source = "../child"
  name   = "from-${var.env}"
}

module "remote" {
  source = "git::https://example.com/mod.git"
}

locals {
  greeting = module.child.greeting
  inner    = module.child.inner
}
`)
	writeTfFile(t, mkdir(t, root, "child"), "main.tf", `
variable "name" {
  default = "default"
}

module "grandchild" {
  source = "./grandchild"
}

output "greeting" {
  value = "hello ${var.name}"
}

output "inner" {
  value = module.grandchild.value
}
`)
	writeTfFile(t, mkdir(t, root, "child", "grandchild"), "main.tf", `
output "value" {
  value = "deep"
}
`)
	vals, err := loadLocalValueMap(context.Background(), parent, models.Environment{Type: "prod", Region: "us-chicago-1", Realm: "oc1"})
	require.NoError(t, err)
	assert.Equal(t, "hello from-prod", vals["greeting"].AsString())
	assert.Equal(t, "deep", vals["inner"].AsString())
}

// A module input that reads a local (or another module's output) waits
// for it to resolve instead of falling back to the module's default.
func TestModuleOutputs_InputsFromLocalsAndModules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	parent := mkdir(t, root, "parent")
	writeTfFile(t, parent, "main.tf", `
module "first" {
  source = "../echo"
  value  = local.region
}

module "second" {
  source = "../echo"
  value  = "${module.first.value}-again"
}

locals {
  region = "us-${var.environment}-1"
  first  = module.first.value
  second = module.second.value
}
`)
	writeTfFile(t, mkdir(t, root, "echo"), "main.tf", `
variable "value" {
  default = "default"
}

output "value" {
  value = var.value
}
`)
	vals, err := loadLocalValueMap(context.Background(), parent, models.Environment{Type: "prod", Region: "us-chicago-1", Realm: "oc1"})
	require.NoError(t, err)
	assert.Equal(t, "us-prod-1", vals["first"].AsString())
	assert.Equal(t, "us-prod-1-again", vals["second"].AsString())
}

// A module calling itself stops at maxModuleDepth instead of recursing
// forever.
func TestModuleOutputs_CycleTerminates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTfFile(t, dir, "main.tf", `
module "self" {
  source = "./"
}

output "x" {
  value = "x"
}
`)
	vals, err := loadLocalValueMap(context.Background(), dir, models.Environment{Type: "dev", Region: "us-test-1", Realm: "test"})
	require.NoError(t, err)
	assert.Equal(t, "x", vals["x"].AsString())
}

func TestLocalFuncs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTfFile(t, dir, "main.tf", `
locals {
  starts    = startswith("oc1:dev:x", "oc1:")
  notstarts = startswith("oc2:dev:x", "oc1:")
  all       = alltrue([true, true])
  notall    = alltrue([true, false])
  none      = alltrue([])
  parts     = split(":", "a:b:c")
  joined    = concat(["a"], ["b"])
  matched   = can(regex("^oc[0-9]+$", "oc1"))
  fallback  = try(local.missing, "fallback")
}
`)
	vals, err := loadLocalValueMap(context.Background(), dir, models.Environment{Type: "dev", Region: "us-test-1", Realm: "test"})
	require.NoError(t, err)
	assert.True(t, vals["starts"].True())
	assert.False(t, vals["notstarts"].True())
	assert.True(t, vals["all"].True())
	assert.False(t, vals["notall"].True())
	assert.True(t, vals["none"].True())
	assert.Equal(t, cty.ListVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b"), cty.StringVal("c")}), vals["parts"])
	assert.Equal(t, 2, vals["joined"].LengthInt())
	assert.True(t, vals["matched"].True())
	assert.Equal(t, "fallback", vals["fallback"].AsString())
}

// alltrue over a list holding an unknown bool is unknown, not a panic.
func TestAllTrue_UnknownElement(t *testing.T) {
	t.Parallel()
	list := cty.ListVal([]cty.Value{cty.True, cty.UnknownVal(cty.Bool)})
	var got cty.Value
	require.NotPanics(t, func() {
		var err error
		got, err = allTrueFunc.Call([]cty.Value{list})
		require.NoError(t, err)
	})
	assert.False(t, got.IsKnown())

	got, err := allTrueFunc.Call([]cty.Value{cty.ListVal([]cty.Value{cty.False, cty.UnknownVal(cty.Bool)})})
	require.NoError(t, err)
	assert.Equal(t, cty.False, got, "a false element before the unknown one decides it")
}

// echoModule writes a module that outputs its input `value`, defaulting
// to "default".
func echoModule(t *testing.T, root string) {
	t.Helper()
	writeTfFile(t, mkdir(t, root, "echo"), "main.tf", `
variable "value" {
  default = "default"
}

output "value" {
  value = var.value
}
`)
}

// A local reading a module output through try() waits for the module
// instead of settling on its fallback while the call is still deferred.
func TestModuleOutputs_TryLocalWaitsForModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	parent := mkdir(t, root, "parent")
	writeTfFile(t, parent, "main.tf", `
module "m" {
  source = "../echo"
  value  = local.region
}

locals {
  region = "us-${var.environment}-1"
  x      = try(module.m.value, "fb")
}
`)
	echoModule(t, root)
	vals, err := loadLocalValueMap(context.Background(), parent, models.Environment{Type: "prod", Region: "us-chicago-1", Realm: "oc1"})
	require.NoError(t, err)
	assert.Equal(t, "us-prod-1", vals["x"].AsString())
}

// A module input reading a local through try() waits for the local
// instead of running the call with the fallback.
func TestModuleOutputs_TryInputWaitsForLocal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	parent := mkdir(t, root, "parent")
	writeTfFile(t, parent, "main.tf", `
module "a" {
  source = "../echo"
  value  = try(local.region, "fb")
}

locals {
  base   = "us-${var.environment}"
  region = "${local.base}-1"
  out    = module.a.value
}
`)
	echoModule(t, root)
	vals, err := loadLocalValueMap(context.Background(), parent, models.Environment{Type: "prod", Region: "us-chicago-1", Realm: "oc1"})
	require.NoError(t, err)
	assert.Equal(t, "us-prod-1", vals["out"].AsString())
}

// An input reading a local that never resolves falls back to the
// variable's default; the call isn't dropped, so its outputs resolve.
func TestModuleOutputs_UnresolvableInputUsesDefault(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	parent := mkdir(t, root, "parent")
	writeTfFile(t, parent, "main.tf", `
module "a" {
  source = "../echo"
  value  = local.broken
}

locals {
  broken = var.missing
  out    = module.a.value
}
`)
	echoModule(t, root)
	vals, err := loadLocalValueMap(context.Background(), parent, models.Environment{Type: "prod", Region: "us-chicago-1", Realm: "oc1"})
	require.NoError(t, err)
	assert.Equal(t, "default", vals["out"].AsString())
}

// Locals that reference each other in a cycle still terminate, and a
// try() in the cycle lands on its fallback.
func TestResolveLocals_CycleFallsBack(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTfFile(t, dir, "main.tf", `
locals {
  a = try(local.b, "fa")
  b = local.a
}
`)
	vals, err := loadLocalValueMap(context.Background(), dir, models.Environment{Type: "prod", Region: "us-chicago-1", Realm: "oc1"})
	require.NoError(t, err)
	assert.Equal(t, "fa", vals["a"].AsString())
	assert.Equal(t, "fa", vals["b"].AsString())
}
