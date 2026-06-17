# Tenant Metadata Entry (TUI) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a user key in tenant info (name/internal/note) for an unresolved DedicatedAICluster or ImportedModel row in the TUI, persist it to the metadata file (creating it if absent), and auto-refresh so the row resolves to a friendly name.

**Architecture:** A new `configloader.SaveMetadata` + upsert helper persists `TenantMetadata`. The production loader gains an `UpsertTenantMetadata` method exposed via an optional `loader.TenantMetadataWriter` interface (type-asserted in the TUI, not added to `loader.Composite`, so test fakes are unaffected). A new `EditTenantView` form overlay captures the fields; on save the TUI calls the writer, then resets and re-loads the tenancy group + current category so owner resolution reruns. ImportedModel also gains an `Internal` column for parity with DAC.

**Tech Stack:** Go, Bubble Tea (`charmbracelet/bubbletea`, `bubbles/textinput`), `sigs.k8s.io/yaml`, standard `testing`.

**Spec:** `docs/superpowers/specs/2026-06-16-tenant-metadata-entry-design.md`

---

## File Structure

- `internal/configloader/metadata_save.go` (create) — `SaveMetadata` + `UpsertTenant` (persistence + merge).
- `internal/configloader/metadata_save_test.go` (create) — round-trip + merge tests.
- `pkg/models/imported_model.go` (modify) — add `OwnerState()`.
- `pkg/models/imported_model_test.go` (create) — `OwnerState` test.
- `internal/columns/imported_model.go` (modify) — add `Internal` column, rebalance ratios.
- `internal/infra/loader/interfaces.go` (modify) — add `TenantMetadataWriter` interface.
- `internal/infra/loader/production/production.go` (modify) — add `UpsertTenantMetadata`.
- `internal/infra/loader/production/production_test.go` (create or modify) — writer assertion + upsert behavior.
- `internal/ui/tui/common/view_mode.go` (modify) — add `EditTenantView`.
- `internal/ui/tui/keys/registry.go` (modify) — add `EditTenant` binding + `SortInternal` to ImportedModel; `EditTenant` to DAC + ImportedModel context.
- `internal/ui/tui/model_state.go` (modify) — add `editTenant *editTenantForm` field.
- `internal/ui/tui/edit_tenant.go` (create) — form state, enter/gate, update, view, save + reload commands, messages.
- `internal/ui/tui/edit_tenant_test.go` (create) — gating + form behavior tests.
- `internal/ui/tui/model_update.go` (modify) — route `EditTenantView` to `updateEditTenantView`.
- `internal/ui/tui/model_view.go` (modify) — render `EditTenantView`.
- `internal/ui/tui/reducer_actions.go` (modify) — dispatch `EditTenant` key in `handleItemActions`.

---

## Task 1: `configloader.SaveMetadata` + `UpsertTenant`

**Files:**
- Create: `internal/configloader/metadata_save.go`
- Test: `internal/configloader/metadata_save_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/configloader/metadata_save_test.go`:

```go
package configloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func strptr(s string) *string { return &s }
func boolptr(b bool) *bool    { return &b }

func TestSaveMetadata_YAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "metadata.yaml") // nested dir must be created
	in := &models.Metadata{Tenants: []models.TenantMetadata{
		{ID: "ocid1.tenancy.oc1..abc", Name: strptr("acme"), IsInternal: boolptr(true)},
	}}
	if err := SaveMetadata(path, in); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}
	got, err := LoadMetadata(path)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if len(got.Tenants) != 1 || got.Tenants[0].ID != "ocid1.tenancy.oc1..abc" ||
		got.Tenants[0].Name == nil || *got.Tenants[0].Name != "acme" {
		t.Fatalf("round-trip mismatch: %+v", got.Tenants)
	}
}

func TestSaveMetadata_JSONRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.json")
	in := &models.Metadata{Tenants: []models.TenantMetadata{
		{ID: "ocid1.tenancy.oc1..xyz", Name: strptr("beta"), IsInternal: boolptr(false)},
	}}
	if err := SaveMetadata(path, in); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}
	got, err := LoadMetadata(path)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if len(got.Tenants) != 1 || got.Tenants[0].Name == nil || *got.Tenants[0].Name != "beta" {
		t.Fatalf("round-trip mismatch: %+v", got.Tenants)
	}
}

func TestSaveMetadata_UnsupportedExt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.txt")
	if err := SaveMetadata(path, &models.Metadata{}); err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}

func TestUpsertTenant_AppendThenReplace(t *testing.T) {
	m := &models.Metadata{}
	UpsertTenant(m, models.TenantMetadata{ID: "id-a", Name: strptr("A"), IsInternal: boolptr(true)})
	UpsertTenant(m, models.TenantMetadata{ID: "id-b", Name: strptr("B"), IsInternal: boolptr(false)})
	if len(m.Tenants) != 2 {
		t.Fatalf("expected 2 tenants, got %d", len(m.Tenants))
	}
	// Replace id-a in place.
	UpsertTenant(m, models.TenantMetadata{ID: "id-a", Name: strptr("A2"), IsInternal: boolptr(false)})
	if len(m.Tenants) != 2 {
		t.Fatalf("replace should not append: got %d", len(m.Tenants))
	}
	if m.Tenants[0].ID != "id-a" || m.Tenants[0].Name == nil || *m.Tenants[0].Name != "A2" {
		t.Fatalf("expected id-a replaced in place: %+v", m.Tenants[0])
	}
	_ = os.Stdout // keep os import if trimmed by tooling
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/configloader/ -run 'SaveMetadata|UpsertTenant' -v`
Expected: FAIL — `undefined: SaveMetadata` / `undefined: UpsertTenant`.

- [ ] **Step 3: Write the implementation**

Create `internal/configloader/metadata_save.go`:

```go
package configloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/jingle2008/toolkit/pkg/models"
)

// SaveMetadata writes m to path, choosing JSON or YAML by the file
// extension. Parent directories are created if missing. Mirrors
// LoadMetadata's extension contract.
func SaveMetadata(path string, m *models.Metadata) error {
	ext := strings.ToLower(filepath.Ext(path))
	var (
		data []byte
		err  error
	)
	switch ext {
	case ".json":
		data, err = json.MarshalIndent(m, "", "  ")
	case ".yaml", ".yml":
		data, err = yaml.Marshal(m)
	default:
		return fmt.Errorf("unsupported metadata file extension: %s", ext)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
		return fmt.Errorf("failed to create metadata dir: %w", mkErr)
	}
	if wErr := os.WriteFile(path, data, 0o600); wErr != nil {
		return fmt.Errorf("failed to write metadata file: %w", wErr)
	}
	return nil
}

// UpsertTenant merges entry into m: if a tenant with the same ID
// already exists it is replaced in place, otherwise entry is appended.
func UpsertTenant(m *models.Metadata, entry models.TenantMetadata) {
	for i := range m.Tenants {
		if m.Tenants[i].ID == entry.ID {
			m.Tenants[i] = entry
			return
		}
	}
	m.Tenants = append(m.Tenants, entry)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/configloader/ -run 'SaveMetadata|UpsertTenant' -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/configloader/metadata_save.go internal/configloader/metadata_save_test.go
git commit -m "feat(configloader): add SaveMetadata + UpsertTenant"
```

---

## Task 2: ImportedModel `OwnerState()` + `Internal` column

**Files:**
- Modify: `pkg/models/imported_model.go`
- Test: `pkg/models/imported_model_test.go`
- Modify: `internal/columns/imported_model.go`
- Modify: `internal/ui/tui/keys/registry.go:250-252`

- [ ] **Step 1: Write the failing test for OwnerState**

Create `pkg/models/imported_model_test.go`:

```go
package models

import "testing"

func TestImportedModel_OwnerState(t *testing.T) {
	var nilOwner ImportedModel
	if got := nilOwner.OwnerState(); got != "" {
		t.Fatalf("nil owner: want %q, got %q", "", got)
	}
	internal := ImportedModel{Owner: &Tenant{IsInternal: true}}
	if got := internal.OwnerState(); got != "true" {
		t.Fatalf("internal owner: want %q, got %q", "true", got)
	}
	external := ImportedModel{Owner: &Tenant{IsInternal: false}}
	if got := external.OwnerState(); got != "false" {
		t.Fatalf("external owner: want %q, got %q", "false", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/models/ -run TestImportedModel_OwnerState -v`
Expected: FAIL — `m.OwnerState undefined`.

- [ ] **Step 3: Add `OwnerState` to ImportedModel**

In `pkg/models/imported_model.go`, after the `TenancyOCID` method (around line 55), add:

```go
// OwnerState returns the owner's internal/external state ("true" /
// "false"), or "" when the owning tenant is unresolved. Mirrors
// DedicatedAICluster.OwnerState.
func (m ImportedModel) OwnerState() string {
	if m.Owner != nil {
		return fmt.Sprint(m.Owner.IsInternal)
	}
	return ""
}
```

(`fmt` is already imported in this file.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/models/ -run TestImportedModel_OwnerState -v`
Expected: PASS.

- [ ] **Step 5: Add the Internal column and rebalance ratios**

In `internal/columns/imported_model.go`, replace the column slice so an `Internal` column sits after `Tenant`, and ratios still sum to 1.00. New ratios: Name 0.20, Tenant 0.20, Internal 0.08, Namespace 0.13, Display Name 0.23, Vendor 0.10, Status 0.06.

Replace the `var ImportedModelColumns = ...` block with:

```go
var ImportedModelColumns = GroupedSet[models.ImportedModel]{Columns: []GroupedColumn[models.ImportedModel]{
	{
		Title: "Name", Key: "name", Ratio: 0.20, TruncateMiddle: true,
		Render: func(_ string, m models.ImportedModel) string { return m.Name },
		RenderForExport: func(realm, region string, _ string, m models.ImportedModel) string {
			return m.OCID(realm, region)
		},
	},
	{
		Title: "Tenant", Key: "tenant", Ratio: 0.20, TruncateMiddle: true,
		Render: func(k string, _ models.ImportedModel) string { return k },
		RenderForExport: func(realm, _ string, _ string, m models.ImportedModel) string {
			return m.TenancyOCID(realm)
		},
	},
	{
		Title: "Internal", Key: "internal", Ratio: 0.08,
		Render: func(_ string, m models.ImportedModel) string { return m.OwnerState() },
	},
	{
		Title: "Namespace", Key: "namespace", Ratio: 0.13, TruncateMiddle: true,
		Render: func(_ string, m models.ImportedModel) string { return m.Namespace },
	},
	{
		Title: "Display Name", Key: "display-name", Ratio: 0.23,
		Render: func(_ string, m models.ImportedModel) string { return m.DisplayName },
	},
	{
		Title: "Vendor", Key: "vendor", Ratio: 0.10,
		Render: func(_ string, m models.ImportedModel) string { return m.Vendor },
	},
	{
		Title: "Status", Key: "status", Ratio: 0.06,
		Render: func(_ string, m models.ImportedModel) string { return m.Status },
	},
}}
```

- [ ] **Step 6: Add SortInternal key to ImportedModel context**

In `internal/ui/tui/keys/registry.go`, change the ImportedModel entry (currently line ~250):

```go
	domain.ImportedModel: {
		common.ListView: {SortTenant, SortInternal, SortVendor, CopyTenant, Refresh},
	},
```

- [ ] **Step 7: Run column + keys tests**

Run: `go test ./internal/columns/... ./internal/ui/tui/keys/... ./pkg/models/ -v`
Expected: PASS — the column ratio-sum invariant test still passes (sum == 1.00) and the keys registry conflict test passes (`SortInternal` = `I`, no clash with the existing ImportedModel bindings `T`/`V`/`t`/`r`).

- [ ] **Step 8: Commit**

```bash
git add pkg/models/imported_model.go pkg/models/imported_model_test.go internal/columns/imported_model.go internal/ui/tui/keys/registry.go
git commit -m "feat(columns): add Internal column + OwnerState to ImportedModel"
```

---

## Task 3: Loader `TenantMetadataWriter` + `UpsertTenantMetadata`

**Files:**
- Modify: `internal/infra/loader/interfaces.go`
- Modify: `internal/infra/loader/production/production.go`
- Test: `internal/infra/loader/production/production_test.go`

- [ ] **Step 1: Write the failing test**

Create (or append to) `internal/infra/loader/production/production_test.go`:

```go
package production

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/models"
)

func sp(s string) *string { return &s }
func bp(b bool) *bool      { return &b }

func TestNew_ImplementsTenantMetadataWriter(t *testing.T) {
	ld := New(context.Background(), "")
	if _, ok := ld.(loader.TenantMetadataWriter); !ok {
		t.Fatal("production.New(...) must satisfy loader.TenantMetadataWriter")
	}
}

func TestUpsertTenantMetadata_WritesAndReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	ld := New(context.Background(), path).(loader.TenantMetadataWriter)

	if err := ld.UpsertTenantMetadata(models.TenantMetadata{
		ID: "ocid1.tenancy.oc1..abc", Name: sp("acme"), IsInternal: bp(true),
	}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := ld.UpsertTenantMetadata(models.TenantMetadata{
		ID: "ocid1.tenancy.oc1..abc", Name: sp("acme-renamed"), IsInternal: bp(false),
	}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := configloader.LoadMetadata(path)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if len(got.Tenants) != 1 {
		t.Fatalf("want 1 tenant (replace, not append), got %d", len(got.Tenants))
	}
	if got.Tenants[0].Name == nil || *got.Tenants[0].Name != "acme-renamed" {
		t.Fatalf("want replaced name, got %+v", got.Tenants[0])
	}
}

func TestUpsertTenantMetadata_NoPathErrors(t *testing.T) {
	ld := New(context.Background(), "").(loader.TenantMetadataWriter)
	if err := ld.UpsertTenantMetadata(models.TenantMetadata{ID: "x", Name: sp("y"), IsInternal: bp(true)}); err == nil {
		t.Fatal("expected error when no metadata file is configured")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/infra/loader/production/ -run TenantMetadata -v`
Expected: FAIL — `undefined: loader.TenantMetadataWriter`.

- [ ] **Step 3: Add the interface**

In `internal/infra/loader/interfaces.go`, after the `Composite` interface block, add:

```go
/*
TenantMetadataWriter is an OPTIONAL capability: persisting a tenant
metadata entry to the backing metadata file. It is deliberately kept
out of Composite so the many fake loaders used in tests need not
implement it. Callers type-assert a Composite to this interface and
degrade gracefully when the assertion fails.
*/
type TenantMetadataWriter interface {
	// UpsertTenantMetadata merges entry into the metadata file
	// (replacing any entry with the same ID, else appending) and
	// persists it, creating the file if it does not exist.
	UpsertTenantMetadata(entry models.TenantMetadata) error
}
```

- [ ] **Step 4: Implement on the production Client (pointer receiver)**

In `internal/infra/loader/production/production.go`, add (note: **pointer receiver** — it mutates `l.metadata`):

```go
// UpsertTenantMetadata merges entry into the in-memory metadata and
// persists the whole set to the configured metadata file, creating it
// if absent. Pointer receiver: it mutates l.metadata, so the runtime
// type behind a loader.Composite must be *Client for the optional
// loader.TenantMetadataWriter assertion to succeed (production.New
// returns &Client{...}, so it does).
func (l *Client) UpsertTenantMetadata(entry models.TenantMetadata) error {
	if l.metadataFile == "" {
		return errors.New("no metadata file configured")
	}
	if l.metadata == nil {
		l.metadata = &models.Metadata{}
	}
	configloader.UpsertTenant(l.metadata, entry)
	return configloader.SaveMetadata(l.metadataFile, l.metadata)
}
```

Add `"errors"` to the import block in `production.go`.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/infra/loader/production/ -run TenantMetadata -v`
Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/infra/loader/interfaces.go internal/infra/loader/production/production.go internal/infra/loader/production/production_test.go
git commit -m "feat(loader): add optional TenantMetadataWriter + production impl"
```

---

## Task 4: TUI — view mode, key binding, and target helper

**Files:**
- Modify: `internal/ui/tui/common/view_mode.go`
- Modify: `internal/ui/tui/keys/registry.go`
- Create: `internal/ui/tui/edit_tenant.go` (helper only in this task)
- Test: `internal/ui/tui/edit_tenant_test.go`

- [ ] **Step 1: Add the `EditTenantView` view mode**

In `internal/ui/tui/common/view_mode.go`, add to the const block (after `ExportView`):

```go
	// EditTenantView is the view mode for the tenant-metadata entry form.
	EditTenantView
```

And in `String()`, add before `default`:

```go
	case EditTenantView:
		return "EditTenant"
```

- [ ] **Step 2: Add the `EditTenant` key binding + wire to categories**

In `internal/ui/tui/keys/registry.go`, add to the binding `var (...)` block that holds `Refresh` etc.:

```go
	// EditTenant opens the tenant-metadata entry form for an
	// unresolved DAC/ImportedModel row.
	EditTenant = key.NewBinding(
		key.WithKeys("E"),
		key.WithHelp("<shift+e>", "Edit Tenant"),
	)
```

Then add `EditTenant` to the DAC and ImportedModel context entries:

```go
	domain.DedicatedAICluster: {
		common.ListView: {SortTenant, SortInternal, SortUsage, SortSize, SortAge, CopyTenant, EditTenant, Refresh, ToggleFaulty, Delete},
	},
	domain.ImportedModel: {
		common.ListView: {SortTenant, SortInternal, SortVendor, CopyTenant, EditTenant, Refresh},
	},
```

- [ ] **Step 3: Write the failing test for the target helper**

Create `internal/ui/tui/edit_tenant_test.go`:

```go
package tui

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestTenantEditTarget(t *testing.T) {
	realm := "oc1"

	// Unresolved DAC → editable, OCID built from realm + TenantID.
	dac := &models.DedicatedAICluster{Name: "dac1", TenantID: "abc"}
	tgt, ok := tenantEditTarget(dac, realm)
	if !ok {
		t.Fatal("unresolved DAC should be a valid target")
	}
	if tgt.ocid != "ocid1.tenancy.oc1..abc" {
		t.Fatalf("ocid: got %q", tgt.ocid)
	}
	if tgt.tenantID != "abc" {
		t.Fatalf("tenantID: got %q", tgt.tenantID)
	}

	// Resolved DAC (Owner set) → not editable.
	resolved := &models.DedicatedAICluster{Name: "dac2", TenantID: "abc", Owner: &models.Tenant{Name: "acme"}}
	if _, ok := tenantEditTarget(resolved, realm); ok {
		t.Fatal("resolved DAC must not be editable")
	}

	// Orphan (UNKNOWN_TENANCY) → not editable.
	orphan := &models.ImportedModel{TenantID: "UNKNOWN_TENANCY"}
	if _, ok := tenantEditTarget(orphan, realm); ok {
		t.Fatal("orphan tenant must not be editable")
	}

	// Empty TenantID → not editable.
	empty := &models.ImportedModel{TenantID: ""}
	if _, ok := tenantEditTarget(empty, realm); ok {
		t.Fatal("empty TenantID must not be editable")
	}

	// Unresolved ImportedModel → editable.
	im := &models.ImportedModel{TenantID: "xyz"}
	tgt, ok = tenantEditTarget(im, realm)
	if !ok || tgt.ocid != "ocid1.tenancy.oc1..xyz" {
		t.Fatalf("unresolved ImportedModel: ok=%v ocid=%q", ok, tgt.ocid)
	}

	// Unrelated type → not a target.
	if _, ok := tenantEditTarget(&models.GPUPool{}, realm); ok {
		t.Fatal("non tenant-owned type must not be a target")
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run TestTenantEditTarget -v`
Expected: FAIL — `undefined: tenantEditTarget`.

- [ ] **Step 5: Create the helper in `edit_tenant.go`**

Create `internal/ui/tui/edit_tenant.go`:

```go
// Package tui — tenant-metadata entry form (EditTenantView).
//
// Lets the user attach a friendly name / internal flag / note to an
// UNRESOLVED DedicatedAICluster or ImportedModel row, persist it to the
// metadata file via loader.TenantMetadataWriter, and auto-refresh so
// the row resolves.
package tui

import (
	"github.com/jingle2008/toolkit/pkg/models"
)

// editTarget identifies the tenant a row points at and whether it can
// be edited (unresolved + has a real tenancy id).
type editTarget struct {
	ocid     string // full tenancy OCID — the metadata entry key
	tenantID string // raw TenantID suffix, for display context
}

// tenantEditTarget inspects a selected item and returns an editTarget
// when it is an unresolved tenant-owned row (DAC or ImportedModel with
// Owner == nil and a real, non-orphan TenantID). ok is false otherwise.
func tenantEditTarget(item any, realm string) (editTarget, bool) {
	var (
		ocid, tenantID string
		resolved       bool
	)
	switch v := item.(type) {
	case *models.DedicatedAICluster:
		if v == nil {
			return editTarget{}, false
		}
		ocid, tenantID, resolved = v.TenancyOCID(realm), v.TenantID, v.Owner != nil
	case *models.ImportedModel:
		if v == nil {
			return editTarget{}, false
		}
		ocid, tenantID, resolved = v.TenancyOCID(realm), v.TenantID, v.Owner != nil
	default:
		return editTarget{}, false
	}
	if resolved || tenantID == "" || tenantID == "UNKNOWN_TENANCY" {
		return editTarget{}, false
	}
	return editTarget{ocid: ocid, tenantID: tenantID}, true
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/ui/tui/ -run TestTenantEditTarget -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/tui/common/view_mode.go internal/ui/tui/keys/registry.go internal/ui/tui/edit_tenant.go internal/ui/tui/edit_tenant_test.go
git commit -m "feat(tui): add EditTenantView mode, EditTenant key, tenant target helper"
```

---

## Task 5: TUI — form state, update, view, save + reload

**Files:**
- Modify: `internal/ui/tui/model_state.go:111` (add field)
- Modify: `internal/ui/tui/edit_tenant.go` (form + commands + messages)
- Modify: `internal/ui/tui/model_update.go:67-81` (route)
- Modify: `internal/ui/tui/model_view.go:177-181` (render)
- Modify: `internal/ui/tui/reducer_actions.go:42-62` (dispatch key)
- Test: `internal/ui/tui/edit_tenant_test.go` (extend)

- [ ] **Step 1: Add the form-state field to the Model**

In `internal/ui/tui/model_state.go`, inside `type Model struct`, after the `dirPicker *filepicker.Model` field (line ~112), add:

```go
	// Tenant-metadata entry form state (EditTenantView).
	editTenant *editTenantForm
```

- [ ] **Step 2: Write the failing test for form open/gating + toggle**

Append to `internal/ui/tui/edit_tenant_test.go`:

```go
import (
	// add alongside existing imports:
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

func TestEnterEditTenantView_GatingAndOpen(t *testing.T) {
	m := makeTestModel(t)
	m.environment.Realm = "oc1"

	// Resolved row → no form, stays in list view.
	m.editTenant = nil
	m.viewMode = common.ListView
	_ = m.openTenantForm(&models.DedicatedAICluster{Name: "d", TenantID: "abc", Owner: &models.Tenant{Name: "acme"}})
	if m.viewMode != common.ListView || m.editTenant != nil {
		t.Fatal("resolved row must not open the form")
	}

	// Unresolved row → form opens, seeded with internal=true default.
	_ = m.openTenantForm(&models.DedicatedAICluster{Name: "d", TenantID: "abc"})
	if m.viewMode != common.EditTenantView || m.editTenant == nil {
		t.Fatal("unresolved row should open the form")
	}
	if m.editTenant.ocid != "ocid1.tenancy.oc1..abc" {
		t.Fatalf("form ocid: got %q", m.editTenant.ocid)
	}
	if !m.editTenant.isInternal {
		t.Fatal("internal should default to true")
	}

	// Toggle internal.
	m.editTenant.toggleInternal()
	if m.editTenant.isInternal {
		t.Fatal("toggleInternal should flip to false")
	}
}

func TestEditTenantForm_EntryRequiresName(t *testing.T) {
	f := newEditTenantForm(editTarget{ocid: "ocid1.tenancy.oc1..abc", tenantID: "abc"})
	if _, ok := f.toEntry(); ok {
		t.Fatal("empty name must be rejected")
	}
	f.name.SetValue("acme")
	f.note.SetValue("hi")
	f.isInternal = true
	entry, ok := f.toEntry()
	if !ok {
		t.Fatal("valid form should produce an entry")
	}
	if entry.ID != "ocid1.tenancy.oc1..abc" || entry.Name == nil || *entry.Name != "acme" ||
		entry.IsInternal == nil || !*entry.IsInternal || entry.Note == nil || *entry.Note != "hi" {
		t.Fatalf("entry mismatch: %+v", entry)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run 'EditTenant' -v`
Expected: FAIL — `undefined: openTenantForm` / `newEditTenantForm`.

- [ ] **Step 4: Implement the form, update, view, and commands**

Append to `internal/ui/tui/edit_tenant.go` (and extend its import block to include the listed packages):

```go
import (
	"errors"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Focus indices for the form fields.
const (
	focusName = iota
	focusInternal
	focusNote
	focusCount
)

type editTenantForm struct {
	ocid       string
	tenantID   string
	name       textinput.Model
	note       textinput.Model
	isInternal bool
	focus      int
}

// tenantSavedMsg / tenantSaveErrMsg report the async upsert result.
type tenantSavedMsg struct{ path string }
type tenantSaveErrMsg struct{ err error }

func newEditTenantForm(t editTarget) *editTenantForm {
	name := textinput.New()
	name.CharLimit = 128
	name.Prompt = ""
	name.Focus()
	note := textinput.New()
	note.CharLimit = 256
	note.Prompt = ""
	return &editTenantForm{
		ocid:       t.ocid,
		tenantID:   t.tenantID,
		name:       name,
		note:       note,
		isInternal: true, // matches getTenants' discovered-tenant default
		focus:      focusName,
	}
}

func (f *editTenantForm) toggleInternal() { f.isInternal = !f.isInternal }

// toEntry builds the TenantMetadata; ok is false when Name is empty.
func (f *editTenantForm) toEntry() (models.TenantMetadata, bool) {
	name := f.name.Value()
	if name == "" {
		return models.TenantMetadata{}, false
	}
	entry := models.TenantMetadata{
		ID:         f.ocid,
		Name:       &name,
		IsInternal: &f.isInternal,
	}
	if note := f.note.Value(); note != "" {
		entry.Note = &note
	}
	return entry, true
}

// cycleFocus moves focus by dir (+1/-1) and updates textinput focus.
func (f *editTenantForm) cycleFocus(dir int) {
	f.focus = (f.focus + dir + focusCount) % focusCount
	if f.focus == focusName {
		f.name.Focus()
	} else {
		f.name.Blur()
	}
	if f.focus == focusNote {
		f.note.Focus()
	} else {
		f.note.Blur()
	}
}

// openTenantForm gates on the selected item and, when editable, opens
// the form. Returns a cmd (toast on rejection, blink on open).
func (m *Model) openTenantForm(item any) tea.Cmd {
	tgt, ok := tenantEditTarget(item, m.environment.Realm)
	if !ok {
		return m.showToast("tenant already resolved or has no tenancy id", toastWarn)
	}
	m.editTenant = newEditTenantForm(tgt)
	m.lastViewMode = m.viewMode
	m.viewMode = common.EditTenantView
	return textinput.Blink
}

// enterEditTenantView is the key-handler entry point.
func (m *Model) enterEditTenantView() tea.Cmd {
	return m.openTenantForm(m.selectedItem())
}

// updateEditTenantView handles key events and async results while the
// form is open.
func (m *Model) updateEditTenantView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tenantSavedMsg:
		m.editTenant = nil
		m.viewMode = common.ListView
		return m, tea.Batch(
			m.showToast(fmt.Sprintf("saved tenant metadata to %s", msg.path), toastInfo),
			m.reloadAfterTenantSave(),
		)
	case tenantSaveErrMsg:
		// Keep the form open so the user's input isn't lost.
		return m, m.showToast(fmt.Sprintf("save failed: %v", msg.err), toastError)
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok || m.editTenant == nil {
		return m, nil
	}
	f := m.editTenant
	switch {
	case key.Matches(keyMsg, keys.Quit) && keyMsg.Type == tea.KeyCtrlC:
		return m, tea.Quit
	case key.Matches(keyMsg, keys.Back):
		m.editTenant = nil
		m.viewMode = common.ListView
		return m, nil
	case keyMsg.Type == tea.KeyTab, keyMsg.Type == tea.KeyDown:
		f.cycleFocus(1)
		return m, nil
	case keyMsg.Type == tea.KeyShiftTab, keyMsg.Type == tea.KeyUp:
		f.cycleFocus(-1)
		return m, nil
	case key.Matches(keyMsg, keys.Confirm):
		entry, valid := f.toEntry()
		if !valid {
			return m, m.showToast("name is required", toastWarn)
		}
		return m, m.saveTenantMetadataCmd(entry)
	case f.focus == focusInternal &&
		(keyMsg.Type == tea.KeySpace || keyMsg.Type == tea.KeyLeft || keyMsg.Type == tea.KeyRight):
		f.toggleInternal()
		return m, nil
	}

	// Route remaining keys to the focused text field.
	var cmd tea.Cmd
	switch f.focus {
	case focusName:
		f.name, cmd = f.name.Update(keyMsg)
	case focusNote:
		f.note, cmd = f.note.Update(keyMsg)
	}
	return m, cmd
}

// saveTenantMetadataCmd persists the entry via the optional loader
// writer interface, off the UI goroutine.
func (m *Model) saveTenantMetadataCmd(entry models.TenantMetadata) tea.Cmd {
	writer, ok := m.loader.(loader.TenantMetadataWriter)
	path := m.metadataPath()
	return func() tea.Msg {
		if !ok {
			return tenantSaveErrMsg{err: errors.New("loader does not support writing metadata")}
		}
		if err := writer.UpsertTenantMetadata(entry); err != nil {
			return tenantSaveErrMsg{err: err}
		}
		return tenantSavedMsg{path: path}
	}
}

// reloadAfterTenantSave resets the tenant-derived data and re-loads the
// tenancy group (rebuilding Tenants from the new metadata) followed by
// the current category, so owner resolution reruns. The Sequence keeps
// Tenants populated before the DAC/ImportedModel map is re-keyed.
func (m *Model) reloadAfterTenantSave() tea.Cmd {
	ds := m.dataset
	if ds == nil {
		return nil
	}
	ds.Tenants = nil
	ds.LimitTenancyOverrideMap = nil
	ds.ConsolePropertyTenancyOverrideMap = nil
	ds.PropertyTenancyOverrideMap = nil
	switch m.category {
	case domain.DedicatedAICluster:
		ds.DedicatedAIClusterMap = nil
	case domain.ImportedModel:
		ds.ImportedModelMap = nil
	default:
		return nil
	}

	m.newLoadContext()
	gen := m.bumpGen()
	grp := loadTenancyOverrideGroupCmd(m.loadCtx, m.loader, m.repoPath, m.environment, gen)
	var cat tea.Cmd
	switch m.category {
	case domain.DedicatedAICluster:
		cat = loadDedicatedAIClustersCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.ImportedModel:
		cat = loadImportedModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	}
	// One beginTask per load to keep pendingTasks balanced; the first
	// returns the spinner cmd, the second returns nil.
	spin := m.beginTask()
	m.beginTask()
	return tea.Sequence(spin, grp, cat)
}

// editTenantView renders the form overlay.
func (m *Model) editTenantView() string {
	f := m.editTenant
	if f == nil {
		return ""
	}
	marker := func(i int) string {
		if f.focus == i {
			return "> "
		}
		return "  "
	}
	internal := "external"
	if f.isInternal {
		internal = "internal"
	}
	var b []string
	b = append(b,
		fmt.Sprintf("Set tenant info for %s", f.tenantID),
		"",
		marker(focusName)+"Name:     "+f.name.View(),
		marker(focusInternal)+"Internal: "+internal+"  (space/←/→ to toggle)",
		marker(focusNote)+"Note:     "+f.note.View(),
		"",
		m.help.ShortHelpView([]key.Binding{
			keys.Confirm, keys.Back,
		}),
	)
	return m.helpBorder.Width(m.viewWidth * 3 / 5).Render(joinLines(b))
}

// joinLines is a tiny helper to avoid importing strings just here.
func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

// metadataPath returns the configured metadata file path for display,
// best-effort via the optional writer; empty when unavailable.
func (m *Model) metadataPath() string {
	if p, ok := m.loader.(interface{ MetadataPath() string }); ok {
		return p.MetadataPath()
	}
	return "metadata file"
}
```

Note: `metadataPath()` uses an optional `MetadataPath()` getter. Add it to the production Client in `production.go`:

```go
// MetadataPath returns the configured metadata file path (for display).
func (l *Client) MetadataPath() string { return l.metadataFile }
```

- [ ] **Step 5: Route the view mode in update and view**

In `internal/ui/tui/model_update.go`, add to the `delegateToActiveView` switch (after the `ExportView` case):

```go
	case common.EditTenantView:
		return m.updateEditTenantView(msg)
```

In `internal/ui/tui/model_view.go`, add to `renderActiveView`'s switch (after the `ExportView` case):

```go
	case common.EditTenantView:
		return m.centered(m.editTenantView())
```

Also in `model_view.go`, `fullHelpView` has a switch on `m.lastViewMode` whose
no-op case group lists the non-list/detail modes. Add `EditTenantView` to that
group so the `exhaustive` linter stays satisfied:

```go
	case common.LoadingView, common.HelpView, common.ExportView, common.EditTenantView:
		// No additional sections for these view modes
```

- [ ] **Step 6: Dispatch the `EditTenant` key**

In `internal/ui/tui/reducer_actions.go`, add a case to the `switch` in `handleItemActions` (after the `CopyTenant` case):

```go
	case key.Matches(msg, keys.EditTenant):
		return m.enterEditTenantView()
```

`enterEditTenantView` ignores `item`/`itemKey` (it re-reads `m.selectedItem()` internally), so no signature change is needed.

- [ ] **Step 7: Run the TUI tests**

Run: `go test ./internal/ui/tui/ -run 'EditTenant' -v`
Expected: PASS (target, gating/open, entry-requires-name tests).

- [ ] **Step 8: Build + full package tests**

Run: `go build ./... && go test ./internal/ui/tui/... ./internal/ui/tui/keys/... ./internal/ui/tui/common/...`
Expected: PASS. (The `common.ViewMode` String test may assert known values — if a case for `EditTenantView` is needed there, add `{EditTenantView, "EditTenant"}` to that table test.)

- [ ] **Step 9: Commit**

```bash
git add internal/ui/tui/ internal/infra/loader/production/production.go
git commit -m "feat(tui): tenant-metadata entry form with save + auto-refresh"
```

---

## Task 6: Full verification

- [ ] **Step 1: Run the entire test suite**

Run: `go test ./...`
Expected: PASS across all packages.

- [ ] **Step 2: Run the linter (repo standard)**

Run: `golangci-lint run ./...`
Expected: no new findings. (Watch for `exhaustive` on the new `common.ViewMode` switches — the existing switches use `//nolint` or `// exhaustive:` comments; mirror the neighboring style if flagged.)

- [ ] **Step 3: Manual smoke check (optional, requires a live env)**

Run the TUI, navigate to DAC or ImportedModel, select an unresolved row (raw id shown), press `E`, fill Name + toggle Internal + Note, press `enter`. Confirm: a success toast names the metadata file, the row re-renders with the friendly name, and the file at `~/.config/toolkit/metadata.yaml` contains the new entry.

- [ ] **Step 4: Update the spec status**

Mark the spec `Status: Implemented` in `docs/superpowers/specs/2026-06-16-tenant-metadata-entry-design.md` and commit.

---

## Notes for the implementer

- **GitNexus:** CLAUDE.md asks for `gitnexus_impact`/`gitnexus_detect_changes` before edits, but the MCP tools are not connected in this environment and the index is stale. Work from the live code; if the tools come online, run impact analysis on `ImportedModelColumns`, `findItem`/`selectedItem`, and `production.New` before editing.
- **No `loader.Composite` change:** `UpsertTenantMetadata` lives only on `*production.Client` and the optional `TenantMetadataWriter` interface — do **not** add it to `Composite`, or every fake loader in the test suite breaks.
- **Pointer receiver matters:** `UpsertTenantMetadata` and `MetadataPath` use a pointer receiver; `production.New` already returns `&Client{}`, so the `m.loader.(loader.TenantMetadataWriter)` assertion succeeds. `TestNew_ImplementsTenantMetadataWriter` guards this.
- **Reload ordering:** the tenancy-group load MUST be processed before the DAC/ImportedModel load so `d.Tenants` is populated when `SetDedicatedAIClusterMap`/`SetImportedModelMap` re-key (they call `buildTenantIDSuffixMap` on `d.Tenants`). `tea.Sequence` guarantees this.
```
