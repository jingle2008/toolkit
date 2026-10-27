package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/pkg/models"
)

const testInstanceOCID = "ocid1.instance.oc1.iad.fake"

// stubOCIDResolver makes the node lookup verify any OCID as an instance
// of GPU pool pool-a, without the OCI calls.
func stubOCIDResolver(d *ops.Deps) {
	d.ResolveGPUNode = func(_ context.Context, _ models.Environment, name, ocid string) (*models.GPUNode, error) {
		return &models.GPUNode{Name: name, ID: ocid, NodePool: "pool-a"}, nil
	}
}

// stubPoolInstance makes the node lookup verify any OCID as an instance
// of GPU pool pool-a, and refuse ocid1.instance.stray.
func stubPoolInstance(d *ops.Deps) *[]string {
	performed := stubPerformSeams(d)
	d.ResolveGPUNode = func(_ context.Context, _ models.Environment, name, ocid string) (*models.GPUNode, error) {
		if ocid == "ocid1.instance.stray" {
			return nil, fmt.Errorf("verify instance %s: %w", ocid, oci.ErrNotGPUPoolInstance)
		}
		return &models.GPUNode{Name: name, ID: ocid, NodePool: "pool-a"}, nil
	}
	return performed
}

// With ocid set, the OCID is the target: terminate's confirm_target must
// repeat it, not the free-form node label.
func TestTerminateNode_OCIDConfirmTarget(t *testing.T) {
	t.Parallel()
	d := &ops.Deps{}
	performed := stubPoolInstance(d)
	args := map[string]any{"node": "node-a", "ocid": testInstanceOCID, "confirm": true}

	res := callToolWith(t, d, "terminate_node", with(args, "confirm_target", "node-a"))
	require.True(t, res.IsError, "the node label must not confirm an OCID target")
	assert.Contains(t, resultText(t, res), fmt.Sprintf("set confirm_target to %q", testInstanceOCID))
	require.Empty(t, *performed)

	res = callToolWith(t, d, "terminate_node", with(args, "confirm_target", testInstanceOCID))
	require.False(t, res.IsError, resultText(t, res))
	assert.Equal(t, testInstanceOCID, structured(t, res)["target"])
	assert.Equal(t, []string{"terminate"}, *performed)
}

// A dry run with ocid reports the verified instance and its GPU pool.
func TestNodeTools_OCIDDryRunReportsPool(t *testing.T) {
	t.Parallel()
	d := &ops.Deps{}
	stubPoolInstance(d)
	for _, tool := range []string{"restart_node", "terminate_node"} {
		sc := structured(t, callToolWith(t, d, tool, map[string]any{"node": "node-a", "ocid": testInstanceOCID, "dry_run": true}))
		assert.Equal(t, "DRY_RUN", sc["status"], tool)
		assert.Equal(t, testInstanceOCID, sc["target"], tool)
		assert.Equal(t, map[string]any{"instance_id": testInstanceOCID, "gpu_pool": "pool-a"}, sc["resolved"], tool)
	}
}

// An OCID outside the env's GPU pools fails the lookup: nothing acts,
// and a dry run fails too.
func TestNodeTools_OCIDNotInGPUPoolRefused(t *testing.T) {
	t.Parallel()
	d := &ops.Deps{}
	performed := stubPoolInstance(d)
	args := map[string]any{"node": "node-a", "ocid": "ocid1.instance.stray", "confirm": true}
	for _, tool := range []string{"restart_node", "terminate_node"} {
		a := args
		if tool == "terminate_node" {
			a = with(args, "confirm_target", "ocid1.instance.stray")
		}
		for _, a := range []map[string]any{a, with(a, "dry_run", true)} {
			res := callToolWith(t, d, tool, a)
			require.True(t, res.IsError, tool)
			assert.Contains(t, resultText(t, res), oci.ErrNotGPUPoolInstance.Error(), tool)
		}
	}
	assert.Empty(t, *performed)
}

// auditLogger is a slog.Handler recording the key/values of every info
// and higher record; log through slog.New(rec).
type auditLogger struct{ records []map[string]any }

func (l *auditLogger) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelInfo
}

func (l *auditLogger) Handle(_ context.Context, rec slog.Record) error {
	r := map[string]any{}
	rec.Attrs(func(a slog.Attr) bool {
		r[a.Key] = a.Value.Any()
		return true
	})
	l.records = append(l.records, r)
	return nil
}

func (l *auditLogger) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *auditLogger) WithGroup(string) slog.Handler      { return l }

// The audit records name the OCID as the target, with the node label
// beside it.
func TestTerminateNode_OCIDAudited(t *testing.T) {
	t.Parallel()
	d := &ops.Deps{}
	stubPoolInstance(d)
	rec := &auditLogger{}
	s := &Server{cfg: config.Config{EnvType: "dev", EnvRegion: "us-ashburn-1", EnvRealm: "oc1"}, logger: slog.New(rec), deps: d}
	in := terminateNodeInput{Node: "node-a", OCID: testInstanceOCID}
	in.Confirm, in.ConfirmTarget = true, testInstanceOCID

	_, out, err := s.handleTerminateNode(context.Background(), nil, in)
	require.NoError(t, err)
	assert.Equal(t, testInstanceOCID, out.Target)
	require.Len(t, rec.records, 2, "begin and done")
	for _, r := range rec.records {
		assert.Equal(t, testInstanceOCID, r["target"])
		assert.Equal(t, "node-a", r["node"])
	}
	assert.Equal(t, "done", rec.records[1]["phase"])
}
