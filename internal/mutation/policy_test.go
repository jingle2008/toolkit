package mutation

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
)

// The tier table every surface gates on: irreversible actions need the
// strongest confirmation (TUI capital Y, CLI --yes, MCP confirm_target).
func TestActions_Tiers(t *testing.T) {
	t.Parallel()
	irreversible := map[Action]bool{
		TerminateNode:               true,
		DeleteDAC:                   true,
		DeleteInferenceService:      true,
		ForceDeleteInferenceService: true,
	}
	seen := map[[2]string]bool{}
	for _, a := range Actions() {
		key := [2]string{a.Verb, a.Kind}
		assert.False(t, seen[key], "%s listed twice", a)
		seen[key] = true
		assert.Equal(t, irreversible[a], a.Tier == Irreversible, "%s tier", a)
	}
	assert.Len(t, Actions(), 11)
}

func TestLookup(t *testing.T) {
	t.Parallel()
	a, ok := Lookup("force-delete", "inference_service")
	require.True(t, ok)
	assert.Equal(t, ForceDeleteInferenceService, a)
	_, ok = Lookup("delete", "node")
	assert.False(t, ok)
}

func TestInferenceServiceDelete(t *testing.T) {
	t.Parallel()
	assert.Equal(t, DeleteInferenceService, InferenceServiceDelete(false))
	assert.Equal(t, ForceDeleteInferenceService, InferenceServiceDelete(true))
}

func TestAction_String(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "terminate node", TerminateNode.String())
}

func fields(e logging.Entry) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(e.Fields); i += 2 {
		m[e.Fields[i].(string)] = e.Fields[i+1] //nolint:forcetypeassert // keys are strings
	}
	return m
}

// The record keeps the field names the CLI and MCP have always written,
// so existing log queries (msg=mutation, phase=…) keep working.
func TestAudit_Record(t *testing.T) {
	t.Parallel()
	sink := logging.NewRingSink(8)
	Audit(sink, "cli", TerminateNode, "n1", PhaseBegin, nil)
	Audit(sink, "mcp", DeleteDAC, "d1", PhaseFailed, errors.New("boom"))
	Audit(sink, "cli", CordonNode, "n1", PhaseDryRun, nil)

	got := sink.Snapshot()
	require.Len(t, got, 3)
	for _, e := range got {
		assert.Equal(t, "mutation", e.Message)
	}
	assert.Equal(t, logging.LevelInfo, got[0].Level)
	assert.Equal(t, map[string]any{
		"action": "terminate", "kind": "node", "target": "n1", "surface": "cli",
		"phase": "begin", "dry_run": false,
	}, fields(got[0]))

	assert.Equal(t, logging.LevelError, got[1].Level)
	f := fields(got[1])
	assert.Equal(t, "failed", f["phase"])
	assert.Equal(t, "mcp", f["surface"])
	assert.EqualError(t, f["error"].(error), "boom") //nolint:forcetypeassert // asserted by EqualError

	assert.Equal(t, true, fields(got[2])["dry_run"])
	assert.Equal(t, "dry_run", fields(got[2])["phase"])
}
