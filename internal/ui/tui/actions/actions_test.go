//nolint:paralleltest // tests use global state → data race under t.Parallel
package actions

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/pkg/models"
)

var (
	clipboardValue string
	clipboardErr   error
)

// monkey-patch clipboard.WriteAll for tests
func init() {
	clipboardWriteAll = func(s string) error {
		clipboardValue = s
		return clipboardErr
	}
}

// fakeLogger is a slog.Handler recording error messages for
// assertions; log through slog.New(logger).
type fakeLogger struct {
	msgs []string
}

func (f *fakeLogger) Enabled(context.Context, slog.Level) bool { return true }

func (f *fakeLogger) Handle(_ context.Context, r slog.Record) error {
	if r.Level == slog.LevelError {
		f.msgs = append(f.msgs, r.Message)
	}
	return nil
}

func (f *fakeLogger) WithAttrs([]slog.Attr) slog.Handler { return f }
func (f *fakeLogger) WithGroup(string) slog.Handler      { return f }

func TestCopyItemName_Nil(t *testing.T) {
	logger := &fakeLogger{}
	CopyItemName(nil, models.Environment{}, slog.New(logger))
	assert.Contains(t, logger.msgs[0], "no item selected")
}

type fakeNamed struct{ name string }

func (f fakeNamed) GetName() string { return f.name }

func TestCopyItemName_NamedItem(t *testing.T) {
	logger := &fakeLogger{}
	clipboardValue = ""
	clipboardErr = nil
	CopyItemName(fakeNamed{"foo"}, models.Environment{}, slog.New(logger))
	assert.Equal(t, "foo", clipboardValue)
}

func TestCopyItemName_DedicatedAICluster(t *testing.T) {
	logger := &fakeLogger{}
	clipboardValue = ""
	clipboardErr = nil
	dac := &models.DedicatedAICluster{Name: "dac-1"}
	env := models.Environment{Realm: "oc1", Region: "me-dubai-1"}
	CopyItemName(dac, env, slog.New(logger))
	assert.Equal(t, "ocid1.generativeaidedicatedaicluster.oc1.me-dubai-1.dac-1", clipboardValue)
}

func TestCopyItemName_ImportedModel(t *testing.T) {
	logger := &fakeLogger{}
	clipboardValue = ""
	clipboardErr = nil
	im := &models.ImportedModel{}
	im.Name = "amaaaaaasxj5imyasw65kzgst7qhopkqbh4hiahgcdpx7gfxesuj7mndycca"
	env := models.Environment{Realm: "oc1", Region: "me-dubai-1"}
	CopyItemName(im, env, slog.New(logger))
	assert.Equal(t,
		"ocid1.generativeaiimportedmodel.oc1.me-dubai-1.amaaaaaasxj5imyasw65kzgst7qhopkqbh4hiahgcdpx7gfxesuj7mndycca",
		clipboardValue)
}

func TestCopyItemName_Unsupported(t *testing.T) {
	logger := &fakeLogger{}
	CopyItemName(123, models.Environment{}, slog.New(logger))
	assert.Contains(t, logger.msgs[len(logger.msgs)-1], "unsupported item type")
}

type fakeTenancyOverride struct{ tenantID string }

func (f fakeTenancyOverride) GetTenantID() string { return f.tenantID }

func TestCopyTenantID_TenancyOverride(t *testing.T) {
	logger := &fakeLogger{}
	clipboardValue = ""
	clipboardErr = nil
	to := fakeTenancyOverride{"tid"}
	CopyTenantID(to, models.Environment{}, slog.New(logger))
	assert.Equal(t, "tid", clipboardValue)
}

func TestCopyTenantID_DedicatedAICluster(t *testing.T) {
	logger := &fakeLogger{}
	clipboardValue = ""
	clipboardErr = nil
	dac := &models.DedicatedAICluster{TenantID: "aaaaaaaaexample"}
	env := models.Environment{Realm: "oc1"}
	CopyTenantID(dac, env, slog.New(logger))
	assert.Equal(t, "ocid1.tenancy.oc1..aaaaaaaaexample", clipboardValue)
}

func TestCopyTenantID_ImportedModel(t *testing.T) {
	logger := &fakeLogger{}
	clipboardValue = ""
	clipboardErr = nil
	im := &models.ImportedModel{}
	im.TenantID = "aaaaaaaaexample"
	env := models.Environment{Realm: "oc1"}
	CopyTenantID(im, env, slog.New(logger))
	assert.Equal(t, "ocid1.tenancy.oc1..aaaaaaaaexample", clipboardValue)
}

func TestCopyTenantID_Nil(t *testing.T) {
	logger := &fakeLogger{}
	CopyTenantID(nil, models.Environment{}, slog.New(logger))
	assert.Contains(t, logger.msgs[0], "no item selected")
}

func TestCopyTenantID_Unsupported(t *testing.T) {
	logger := &fakeLogger{}
	CopyTenantID(123, models.Environment{}, slog.New(logger))
	assert.Contains(t, logger.msgs[len(logger.msgs)-1], "unsupported item type")
}

func TestCopyItemName_ClipboardError(t *testing.T) {
	logger := &fakeLogger{}
	clipboardErr = errors.New("fail")
	CopyItemName(fakeNamed{"foo"}, models.Environment{}, slog.New(logger))
	assert.Contains(t, logger.msgs[len(logger.msgs)-1], "failed to copy name")
	clipboardErr = nil
}

func TestCopyTenantID_ClipboardError(t *testing.T) {
	logger := &fakeLogger{}
	clipboardErr = errors.New("fail")
	to := fakeTenancyOverride{"tid"}
	CopyTenantID(to, models.Environment{}, slog.New(logger))
	assert.Contains(t, logger.msgs[len(logger.msgs)-1], "failed to copy tenantID")
	clipboardErr = nil
}
