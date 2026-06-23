# Set Tenant Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add tenant-metadata upsert to the CLI (`toolkit set tenant <ocid>`) and MCP (`set_tenant` tool), closing the one cross-surface write gap found in the TUI/CLI/MCP parity audit.

**Architecture:** Both surfaces build a `models.TenantMetadata` identical to the TUI's `editTenantForm.toEntry()` and delegate to the existing `loader.TenantMetadataWriter.UpsertTenantMetadata`. No new persistence logic. The CLI reuses the shared `withMutationSetup`/`runMutation` machinery (extended with a `needsEnv` flag because tenant metadata is global, not env-scoped); the MCP tool reuses `runMutationTool` directly (it skips `handleMutation` since there is no env to derive).

**Tech Stack:** Go, Cobra (CLI), `github.com/modelcontextprotocol/go-sdk/mcp` (MCP), testify (MCP tests), gofumpt v0.10.0 + golangci-lint v2.12.2 (pinned via `make setup`).

## Global Constraints

- Upsert only — no delete/remove. Exact TUI parity.
- No new loader code: reuse `loader.TenantMetadataWriter.UpsertTenantMetadata` (signature: `UpsertTenantMetadata(entry models.TenantMetadata) error`).
- `models.TenantMetadata` shape: `ID string` (full tenancy OCID = key), `Name *string` (required), `IsInternal *bool` (default `true`), `Note *string` (optional, set only when non-empty).
- Naming: CLI `toolkit set tenant <ocid>`; MCP tool `set_tenant`. Audit labels `action="set" kind="tenant"`.
- Tenant metadata is **not** env-scoped: do not require `--env-type/region/realm` (CLI) and do not add `envOverride` (MCP).
- Validation (both surfaces): name non-empty; OCID must start with `ocid1.tenancy.`. Fail before any write.
- Confirmation tier: recoverable (not destructive). CLI prompts unless `--yes`; MCP requires `confirm=true`. No `RequireExplicitYes`.
- `production.New(ctx, metadataFile)` returns `loader.Composite` (an interface); type-assert to `loader.TenantMetadataWriter` to reach the write method (mirrors `edit_tenant.go:270`).
- Run `make ci` (uses pinned gofumpt/golangci-lint) before each commit; it must pass.
- Per repo mandate (CLAUDE.md): before editing `withMutationSetup` / `validateMutationConfig`, run `gitnexus_impact({target, direction:"upstream"})` and report the blast radius; run `gitnexus_detect_changes()` before the final commit.

---

### Task 1: Extend the CLI mutation prelude with a `needsEnv` flag

Tenant metadata needs no env triple. Add a `needsEnv bool` parameter to `validateMutationConfig` and `withMutationSetup`; existing callers pass `true`, the new command (Task 2) passes `false`.

**Files:**
- Modify: `internal/cli/mutate.go` (`validateMutationConfig` ~line 61, `withMutationSetup` ~line 97)
- Modify (call sites, all pass `true` as new 4th positional arg to `withMutationSetup`):
  - `internal/cli/cordon.go:66`
  - `internal/cli/drain.go:31`
  - `internal/cli/reboot.go:38`
  - `internal/cli/terminate.go:41`
  - `internal/cli/scale.go:46`
  - `internal/cli/delete_dac.go:41`
- Test: `internal/cli/mutate_test.go` (create if absent; else append)

**Interfaces:**
- Produces: `validateMutationConfig(cfg config.Config, needsKube, needsRepo, needsEnv bool) error` and `withMutationSetup(cfgFile *string, needsKube, needsRepo, needsEnv bool, fn func(ctx context.Context, cfg config.Config, env models.Environment) error) error` — Task 2 calls `withMutationSetup` with `needsEnv=false`.

- [ ] **Step 1: Run impact analysis (repo mandate)**

Run (MCP tool): `gitnexus_impact({target: "withMutationSetup", direction: "upstream"})` and `gitnexus_impact({target: "validateMutationConfig", direction: "upstream"})`. Report direct callers + risk level. Expected: the 6 call sites listed above; if anything else appears, stop and reconcile before editing. (If the index is stale, run `npx gitnexus analyze` first.)

- [ ] **Step 2: Write the failing test**

Add to `internal/cli/mutate_test.go`:

```go
//nolint:paralleltest // viper global state
package cli

import (
	"testing"

	"github.com/jingle2008/toolkit/internal/config"
)

func TestValidateMutationConfig_NeedsEnvFalse_SkipsEnvTriple(t *testing.T) {
	// No env type/region/realm set, but needsEnv=false → must pass.
	cfg := config.Config{}
	if err := validateMutationConfig(cfg, false, false, false); err != nil {
		t.Fatalf("needsEnv=false should not require env triple, got: %v", err)
	}
}

func TestValidateMutationConfig_NeedsEnvTrue_RequiresEnvTriple(t *testing.T) {
	cfg := config.Config{}
	err := validateMutationConfig(cfg, false, false, true)
	if err == nil {
		t.Fatal("needsEnv=true with empty env must error")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestValidateMutationConfig -v`
Expected: compile failure — `validateMutationConfig` takes 3 args, not 4 (`too many arguments`).

- [ ] **Step 4: Add the `needsEnv` parameter to `validateMutationConfig`**

In `internal/cli/mutate.go`, change the signature and guard the env checks. Replace:

```go
func validateMutationConfig(cfg config.Config, needsKube, needsRepo bool) error {
	var missing []string
	if needsRepo && cfg.RepoPath == "" {
		missing = append(missing, "--repo-path")
	}
	if cfg.EnvType == "" {
		missing = append(missing, "--env-type")
	}
	if cfg.EnvRegion == "" {
		missing = append(missing, "--env-region")
	}
	if cfg.EnvRealm == "" {
		missing = append(missing, "--env-realm")
	}
```

with:

```go
func validateMutationConfig(cfg config.Config, needsKube, needsRepo, needsEnv bool) error {
	var missing []string
	if needsRepo && cfg.RepoPath == "" {
		missing = append(missing, "--repo-path")
	}
	if needsEnv {
		if cfg.EnvType == "" {
			missing = append(missing, "--env-type")
		}
		if cfg.EnvRegion == "" {
			missing = append(missing, "--env-region")
		}
		if cfg.EnvRealm == "" {
			missing = append(missing, "--env-realm")
		}
	}
```

- [ ] **Step 5: Thread `needsEnv` through `withMutationSetup`**

In `internal/cli/mutate.go`, change `withMutationSetup`'s signature and its call to `validateMutationConfig`. Replace:

```go
func withMutationSetup(
	cfgFile *string,
	needsKube, needsRepo bool,
	fn func(ctx context.Context, cfg config.Config, env models.Environment) error,
) error {
	if err := readConfigFile(cfgFile); err != nil {
		return err
	}
	var cfg config.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}
	if err := validateMutationConfig(cfg, needsKube, needsRepo); err != nil {
		return err
	}
```

with:

```go
func withMutationSetup(
	cfgFile *string,
	needsKube, needsRepo, needsEnv bool,
	fn func(ctx context.Context, cfg config.Config, env models.Environment) error,
) error {
	if err := readConfigFile(cfgFile); err != nil {
		return err
	}
	var cfg config.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}
	if err := validateMutationConfig(cfg, needsKube, needsRepo, needsEnv); err != nil {
		return err
	}
```

- [ ] **Step 6: Update the 6 existing call sites to pass `true`**

Add `true` as the new 4th positional argument (after `needsRepo`) in each:
- `cordon.go:66`: `withMutationSetup(cfgFile, true, false, true, func(...`
- `drain.go:31`: `withMutationSetup(cfgFile, true, false, true, func(...`
- `reboot.go:38`: `withMutationSetup(cfgFile, needsKube, false, true, func(...`
- `terminate.go:41`: `withMutationSetup(cfgFile, needsKube, false, true, func(...`
- `scale.go:46`: `withMutationSetup(cfgFile, true, true, true, func(...`
- `delete_dac.go:41`: `withMutationSetup(cfgFile, false, false, true, func(...`

- [ ] **Step 7: Run the new test + full CLI suite**

Run: `go test ./internal/cli/ -count=1`
Expected: PASS (new validate tests pass; all existing mutation tests still pass — they set the env triple and pass `needsEnv=true`).

- [ ] **Step 8: Commit**

```bash
make ci
git add internal/cli/mutate.go internal/cli/cordon.go internal/cli/drain.go internal/cli/reboot.go internal/cli/terminate.go internal/cli/scale.go internal/cli/delete_dac.go internal/cli/mutate_test.go
git commit -m "refactor(cli): add needsEnv flag to mutation prelude

Tenant-metadata mutations are keyed by full OCID in a global file and
need no env triple. Existing 6 call sites pass needsEnv=true.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 2: CLI `toolkit set tenant <ocid>` command

**Files:**
- Create: `internal/cli/set_tenant.go`
- Modify: `internal/cli/root.go:75` (register after `addTerminateCommand`)
- Test: `internal/cli/set_tenant_test.go`

**Interfaces:**
- Consumes: `withMutationSetup(cfgFile, false, false, false, fn)` and `runMutation(ctx, in, out, mutationPlan{...}, perform)` from Task 1 / `mutate.go`.
- Produces: `addSetCommand(rootCmd *cobra.Command, cfgFile *string)` (called from root.go); package-level seam `var setTenantFn func(ctx context.Context, cfg config.Config, entry models.TenantMetadata) error`.

- [ ] **Step 1: Write the failing tests**

Create `internal/cli/set_tenant_test.go`:

```go
//nolint:paralleltest // NewRootCmd uses cobra global state and viper singleton
package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
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
	stageTenantEnv(t)
	called := false
	defer swap(&setTenantFn, func(context.Context, config.Config, models.TenantMetadata) error {
		called = true
		return nil
	})()

	out, err := runRootCmd(t, []string{"set", "tenant", testTenancyOCID, "--name", "Acme", "--dry-run"}, "")
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
	stageTenantEnv(t)
	defer swap(&setTenantFn, func(context.Context, config.Config, models.TenantMetadata) error {
		t.Fatal("must not write when --name missing")
		return nil
	})()

	_, err := runRootCmd(t, []string{"set", "tenant", testTenancyOCID, "--yes"}, "")
	if err == nil {
		t.Fatal("expected error when --name missing")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention name: %v", err)
	}
}

func TestSetTenant_RejectsNonTenancyOCID(t *testing.T) {
	stageTenantEnv(t)
	defer swap(&setTenantFn, func(context.Context, config.Config, models.TenantMetadata) error {
		t.Fatal("must not write for bad OCID")
		return nil
	})()

	_, err := runRootCmd(t, []string{"set", "tenant", "not-an-ocid", "--name", "Acme", "--yes"}, "")
	if err == nil {
		t.Fatal("expected error for non-tenancy OCID")
	}
	if !strings.Contains(err.Error(), "ocid1.tenancy.") {
		t.Errorf("error should mention the expected prefix: %v", err)
	}
}

func TestSetTenant_YesWritesEntry(t *testing.T) {
	stageTenantEnv(t)
	var got models.TenantMetadata
	defer swap(&setTenantFn, func(_ context.Context, _ config.Config, e models.TenantMetadata) error {
		got = e
		return nil
	})()

	out, err := runRootCmd(t, []string{
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

func TestSetTenant_DefaultInternalTrue_NoteOmittedWhenEmpty(t *testing.T) {
	stageTenantEnv(t)
	var got models.TenantMetadata
	defer swap(&setTenantFn, func(_ context.Context, _ config.Config, e models.TenantMetadata) error {
		got = e
		return nil
	})()

	if _, err := runRootCmd(t, []string{"set", "tenant", testTenancyOCID, "--name", "Acme", "--yes"}, ""); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got.IsInternal == nil || *got.IsInternal != true {
		t.Errorf("IsInternal should default true, got %v", got.IsInternal)
	}
	if got.Note != nil {
		t.Errorf("Note should be nil when --note empty, got %v", *got.Note)
	}
}

func TestSetTenant_PerformError(t *testing.T) {
	stageTenantEnv(t)
	defer swap(&setTenantFn, func(context.Context, config.Config, models.TenantMetadata) error {
		return errors.New("disk full")
	})()

	_, err := runRootCmd(t, []string{"set", "tenant", testTenancyOCID, "--name", "Acme", "--yes"}, "")
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("expected wrapped perform error, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run TestSetTenant -v`
Expected: compile failure — `undefined: setTenantFn` and unknown command `set`.

- [ ] **Step 3: Implement the command**

Create `internal/cli/set_tenant.go`:

```go
package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/pkg/models"
)

// tenancyOCIDPrefix guards against entries that would never resolve:
// Metadata.GetTenants keys off the OCID's realm segment.
const tenancyOCIDPrefix = "ocid1.tenancy."

// setTenantFn is the seam tests use to fake the metadata write.
// Production builds a fresh loader and upserts via the optional
// TenantMetadataWriter capability (same path the TUI uses).
var setTenantFn = func(ctx context.Context, cfg config.Config, entry models.TenantMetadata) error {
	ld := production.New(ctx, cfg.MetadataFile)
	writer, ok := ld.(loader.TenantMetadataWriter)
	if !ok {
		return errors.New("loader does not support writing metadata")
	}
	return writer.UpsertTenantMetadata(entry)
}

func addSetCommand(rootCmd *cobra.Command, cfgFile *string) {
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Create or update a resource",
	}

	var (
		name     string
		internal bool
		note     string
		dryRun   bool
		yes      bool
	)
	tenantCmd := &cobra.Command{
		Use:   "tenant <ocid>",
		Short: "Set tenant metadata (name / internal flag / note) by tenancy OCID",
		Long: `Create or replace the metadata entry for a tenancy OCID in the
metadata file (created if absent). This is the headless equivalent of the
TUI's "edit tenant" form. The entry is keyed by the full tenancy OCID and
stored globally, so no --env flags are required.

<ocid> must be a full tenancy OCID (starts with ` + "`ocid1.tenancy.`" + `).

Examples:
  toolkit set tenant ocid1.tenancy.oc1..aaaa --name "Acme Corp" --yes
  toolkit set tenant ocid1.tenancy.oc1..aaaa --name Acme --internal=false --note vip -y`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ocid := args[0]
			if strings.TrimSpace(name) == "" {
				return errors.New("--name is required")
			}
			if !strings.HasPrefix(ocid, tenancyOCIDPrefix) {
				return fmt.Errorf("invalid tenancy OCID %q: must start with %q", ocid, tenancyOCIDPrefix)
			}
			return withMutationSetup(cfgFile, false, false, false, func(ctx context.Context, cfg config.Config, _ models.Environment) error {
				return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), mutationPlan{
					Action:  "set",
					Kind:    "tenant",
					Target:  ocid,
					Surface: "cli",
					DryRun:  dryRun,
					Yes:     yes,
				}, func(ctx context.Context) error {
					nameVal, internalVal := name, internal
					entry := models.TenantMetadata{
						ID:         ocid,
						Name:       &nameVal,
						IsInternal: &internalVal,
					}
					if note != "" {
						noteVal := note
						entry.Note = &noteVal
					}
					return setTenantFn(ctx, cfg, entry)
				})
			})
		},
	}
	tenantCmd.Flags().StringVar(&name, "name", "", "Friendly tenant name (required)")
	tenantCmd.Flags().BoolVar(&internal, "internal", true, "Mark the tenant internal (--internal=false for external)")
	tenantCmd.Flags().StringVar(&note, "note", "", "Optional free-form note")
	tenantCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "Print what would happen and exit")
	tenantCmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the interactive confirmation prompt")

	setCmd.AddCommand(tenantCmd)
	rootCmd.AddCommand(setCmd)
}
```

- [ ] **Step 4: Register the command in root.go**

In `internal/cli/root.go`, add after line 75 (`addTerminateCommand(rootCmd, &cfgFile)`):

```go
	addSetCommand(rootCmd, &cfgFile)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestSetTenant -v`
Expected: PASS (all six tests).

- [ ] **Step 6: Commit**

```bash
make ci
git add internal/cli/set_tenant.go internal/cli/set_tenant_test.go internal/cli/root.go
git commit -m "feat(cli): add 'set tenant' command for tenant metadata

Headless equivalent of the TUI edit-tenant form. Upserts a
TenantMetadata entry by tenancy OCID via the existing
TenantMetadataWriter. No env flags required (global, OCID-keyed file).

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 3: MCP `set_tenant` tool

**Files:**
- Modify: `internal/mcp/mutations.go` (add seam, input type, handler, registration; add `strings` + `loader` imports)
- Test: `internal/mcp/mutations_test.go` (add tests; extend the two enumeration tests + `stubAllMutationSeams`)

**Interfaces:**
- Consumes: `s.runMutationTool(ctx, req, action, kind, target, confirm, perform)` and `s.loader loader.Composite` from `server.go`/`mutations.go`.
- Produces: package-level seam `var mcpUpsertTenantFn func(s *Server, entry models.TenantMetadata) error`; tool `set_tenant`.

- [ ] **Step 1: Write the failing tests**

Add to `internal/mcp/mutations_test.go`:

```go
func TestIntegration_SetTenantTool_ConfirmTrueExecutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	var got models.TenantMetadata
	orig := mcpUpsertTenantFn
	defer func() { mcpUpsertTenantFn = orig }()
	mcpUpsertTenantFn = func(_ *Server, e models.TenantMetadata) error {
		got = e
		return nil
	}

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec)

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name: "set_tenant",
		Arguments: map[string]any{
			"ocid":    "ocid1.tenancy.oc1..aaaa",
			"name":    "Acme",
			"confirm": true,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.IsError)
	if got.ID != "ocid1.tenancy.oc1..aaaa" || got.Name == nil || *got.Name != "Acme" {
		t.Errorf("unexpected entry: %+v", got)
	}
	if got.IsInternal == nil || *got.IsInternal != true {
		t.Errorf("IsInternal should default true, got %v", got.IsInternal)
	}
	msgs := waitForMsgs(t, rec)
	body, _ := msgs[0].Data.(string)
	assert.Contains(t, body, "set tenant/ocid1.tenancy.oc1..aaaa: OK")
}

func TestIntegration_SetTenantTool_RejectsBadOCID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	orig := mcpUpsertTenantFn
	defer func() { mcpUpsertTenantFn = orig }()
	mcpUpsertTenantFn = func(*Server, models.TenantMetadata) error {
		t.Fatal("must not write for bad OCID")
		return nil
	}

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec)

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "set_tenant",
		Arguments: map[string]any{"ocid": "nope", "name": "Acme", "confirm": true},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError, "bad OCID must error")
}

func TestIntegration_SetTenantTool_RequiresName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	orig := mcpUpsertTenantFn
	defer func() { mcpUpsertTenantFn = orig }()
	mcpUpsertTenantFn = func(*Server, models.TenantMetadata) error {
		t.Fatal("must not write when name missing")
		return nil
	}

	rec := &recorder{}
	clientSess := newTestPair(ctx, t, stubLoader{}, rec)

	res, err := clientSess.CallTool(ctx, &sdk.CallToolParams{
		Name:      "set_tenant",
		Arguments: map[string]any{"ocid": "ocid1.tenancy.oc1..aaaa", "confirm": true},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.IsError, "missing name must error")
}
```

- [ ] **Step 2: Extend the two enumeration tests + the seam stub**

In `stubAllMutationSeams` (mutations_test.go ~line 59): capture and restore `mcpUpsertTenantFn` alongside the others, and stub it to mark called:

```go
	oUpsert := mcpUpsertTenantFn
	mcpUpsertTenantFn = func(*Server, models.TenantMetadata) error { mark(); return nil }
```
Add `oUpsert` to the restore closure: `mcpUpsertTenantFn = oUpsert`.

In `TestIntegration_AllMutationTools_RefuseWithoutConfirm` (~line 106) add to the `tools` slice:
```go
		{"set_tenant", map[string]any{"ocid": "ocid1.tenancy.oc1..aaaa", "name": "Acme"}},
```

In `TestIntegration_MutationTools_RegisteredInListTools` (~line 602) add `"set_tenant"` to the expected names slice.

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/mcp/ -run 'TestIntegration_SetTenantTool|RefuseWithoutConfirm|RegisteredInListTools' -v`
Expected: compile failure — `undefined: mcpUpsertTenantFn`.

- [ ] **Step 4: Implement seam, input, handler, registration**

In `internal/mcp/mutations.go`:

(a) Add imports `"errors"`, `"strings"`, and `"github.com/jingle2008/toolkit/internal/infra/loader"` to the import block.

(b) Add the seam to the `var (...)` block (after `mcpDeleteDACFn`):

```go
	mcpUpsertTenantFn = func(s *Server, entry models.TenantMetadata) error {
		writer, ok := s.loader.(loader.TenantMetadataWriter)
		if !ok {
			return errors.New("loader does not support writing metadata")
		}
		return writer.UpsertTenantMetadata(entry)
	}
```

(c) Add the input type (after `deleteDACInput`, ~line 161). Note: NO `envOverride`.

```go
type setTenantInput struct {
	OCID       string `json:"ocid" jsonschema:"the full tenancy OCID (the metadata entry key); must start with ocid1.tenancy."`
	Name       string `json:"name" jsonschema:"friendly tenant name (required)"`
	IsInternal *bool  `json:"is_internal,omitempty" jsonschema:"mark tenant internal; defaults to true when omitted"`
	Note       string `json:"note,omitempty" jsonschema:"optional free-form note"`
	confirmGate
}
```

(d) Add the handler (after `handleDeleteDAC`, ~line 240). It validates, then calls `runMutationTool` directly (no env to derive):

```go
func (s *Server) handleSetTenant(ctx context.Context, req *sdk.CallToolRequest, in setTenantInput) (*sdk.CallToolResult, mutationResult, error) {
	if strings.TrimSpace(in.Name) == "" {
		return failTool[mutationResult](ctx, req, "set tenant", errors.New("name is required"))
	}
	if !strings.HasPrefix(in.OCID, "ocid1.tenancy.") {
		return failTool[mutationResult](ctx, req, "set tenant",
			fmt.Errorf("invalid tenancy OCID %q: must start with ocid1.tenancy.", in.OCID))
	}
	internal := true
	if in.IsInternal != nil {
		internal = *in.IsInternal
	}
	name := in.Name
	entry := models.TenantMetadata{ID: in.OCID, Name: &name, IsInternal: &internal}
	if in.Note != "" {
		note := in.Note
		entry.Note = &note
	}
	return s.runMutationTool(ctx, req, "set", "tenant", in.OCID, in.Confirm, func() error {
		return mcpUpsertTenantFn(s, entry)
	})
}
```

(e) Register in `registerMutationTools` (after the `delete_dac` block). Use a trimmed footer (no env-override clause):

```go
	sdk.AddTool(s.server, &sdk.Tool{
		Name: "set_tenant",
		Description: "Create or update a tenancy's metadata (name / internal flag / note), keyed by full tenancy OCID in the global metadata file." +
			" Mutating: requires confirm=true to execute, otherwise refuses without acting." +
			" Not env-scoped: env_type/env_region/env_realm do not apply.",
	}, s.handleSetTenant)
```

- [ ] **Step 5: Run the MCP suite**

Run: `go test ./internal/mcp/ -count=1`
Expected: PASS (new tests + the extended enumeration tests).

- [ ] **Step 6: Commit**

```bash
make ci
git add internal/mcp/mutations.go internal/mcp/mutations_test.go
git commit -m "feat(mcp): add set_tenant tool for tenant metadata

Eighth mutation tool. Upserts a TenantMetadata entry by tenancy OCID
via the loader's TenantMetadataWriter. Not env-scoped, so it bypasses
handleMutation and calls runMutationTool directly.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 4: Final verification & integration

**Files:** none (verification only).

- [ ] **Step 1: Full CI**

Run: `make ci`
Expected: `0 issues`, all tests pass, coverage ≥ 80%.

- [ ] **Step 2: Manual smoke (dry-run, no live deps)**

Run: `go run ./cmd/toolkit set tenant ocid1.tenancy.oc1..aaaa --name "Test" --dry-run`
Expected stdout: `DRY-RUN: would set tenant/ocid1.tenancy.oc1..aaaa`

Run: `go run ./cmd/toolkit set tenant bad --name "Test" --yes`
Expected: non-zero exit, error mentioning `ocid1.tenancy.`

- [ ] **Step 3: Detect-changes (repo mandate)**

Run (MCP tool): `gitnexus_detect_changes()`. Confirm only the expected symbols/flows changed (CLI `set` command, MCP `set_tenant`, the `withMutationSetup`/`validateMutationConfig` signature). Report anything unexpected.

- [ ] **Step 4: Parity confirmation**

Confirm the audit gap is closed: tenant-metadata upsert now exists on all three surfaces (TUI `shift+e`, CLI `set tenant`, MCP `set_tenant`). No code change in this step — just the closing note for the PR description.

---

## Self-Review

**Spec coverage:**
- Naming (`set tenant` / `set_tenant`) → Tasks 2, 3. ✓
- Env dropped (CLI `needsEnv=false`, MCP no `envOverride`) → Tasks 1, 2, 3. ✓
- Upsert-only, reuse `UpsertTenantMetadata`, no new loader code → Tasks 2, 3 (seams delegate to existing writer). ✓
- Entry shape / IsInternal default true / Note-when-nonempty → Tasks 2 (Step 3, tests), 3 (handler, tests). ✓
- OCID prefix + name validation → Tasks 2, 3. ✓
- Recoverable confirm tier (CLI prompt-unless-`--yes`, MCP `confirm=true`) → reuses `runMutation`/`runMutationTool`; no `RequireExplicitYes`. ✓
- Testing mirrors `delete_dac` / `handleDeleteDAC` → Tasks 2, 3. ✓
- gitnexus impact/detect mandate → Task 1 Step 1, Task 4 Step 3. ✓

**Placeholder scan:** none — every code/test step shows full content.

**Type consistency:** `setTenantFn(ctx, cfg, entry)`, `mcpUpsertTenantFn(s, entry)`, `withMutationSetup(cfgFile, needsKube, needsRepo, needsEnv, fn)`, `validateMutationConfig(cfg, needsKube, needsRepo, needsEnv)`, and `models.TenantMetadata{ID, Name *string, IsInternal *bool, Note *string}` are used identically across all tasks. ✓
