package actions

import (
	"errors"
	"testing"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
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

// fakeLogger records error messages for assertions
type fakeLogger struct {
	msgs []string
}

func (f *fakeLogger) Errorw(msg string, kv ...any) {
	f.msgs = append(f.msgs, msg)
}
func (f *fakeLogger) Debugw(string, ...any)            {}
func (f *fakeLogger) Infow(string, ...any)             {}
func (f *fakeLogger) Sync() error                      { return nil }
func (f *fakeLogger) WithFields(...any) logging.Logger { return f }
func (f *fakeLogger) DebugEnabled() bool               { return false }

func TestCopyItemName_Nil(t *testing.T) {
	t.Parallel()
	logger := &fakeLogger{}
	CopyItemName(nil, &models.Environment{}, logger)
	assert.Contains(t, logger.msgs[0], "no item selected")
}

type fakeNamed struct{ name string }

func (f fakeNamed) GetName() string { return f.name }

func TestCopyItemName_NamedItem(t *testing.T) {
	t.Parallel()
	logger := &fakeLogger{}
	clipboardValue = ""
	clipboardErr = nil
	CopyItemName(fakeNamed{"foo"}, &models.Environment{}, logger)
	assert.Equal(t, "foo", clipboardValue)
}

func TestCopyItemName_DedicatedAICluster(t *testing.T) {
	t.Parallel()
	logger := &fakeLogger{}
	clipboardValue = ""
	clipboardErr = nil
	dac := &models.DedicatedAICluster{}
	env := &models.Environment{Realm: "realm", Region: "region"}
	CopyItemName(dac, env, logger)
}

func TestCopyItemName_Unsupported(t *testing.T) {
	t.Parallel()
	logger := &fakeLogger{}
	CopyItemName(123, &models.Environment{}, logger)
	assert.Contains(t, logger.msgs[len(logger.msgs)-1], "unsupported item type")
}

type fakeTenancyOverride struct{ tenantID string }

func (f fakeTenancyOverride) GetTenantID() string { return f.tenantID }

func TestCopyTenantID_TenancyOverride(t *testing.T) {
	t.Parallel()
	logger := &fakeLogger{}
	clipboardValue = ""
	clipboardErr = nil
	to := fakeTenancyOverride{"tid"}
	CopyTenantID(to, &models.Environment{}, logger)
	assert.Equal(t, "tid", clipboardValue)
}

func TestCopyTenantID_DedicatedAICluster(t *testing.T) {
	t.Parallel()
	logger := &fakeLogger{}
	clipboardValue = ""
	clipboardErr = nil
	dac := &models.DedicatedAICluster{}
	env := &models.Environment{Realm: "realm"}
	CopyTenantID(dac, env, logger)
}

func TestCopyTenantID_Nil(t *testing.T) {
	t.Parallel()
	logger := &fakeLogger{}
	CopyTenantID(nil, &models.Environment{}, logger)
	assert.Contains(t, logger.msgs[0], "no item selected")
}

func TestCopyTenantID_Unsupported(t *testing.T) {
	t.Parallel()
	logger := &fakeLogger{}
	CopyTenantID(123, &models.Environment{}, logger)
	assert.Contains(t, logger.msgs[len(logger.msgs)-1], "unsupported item type")
}

func TestCopyItemName_ClipboardError(t *testing.T) {
	t.Parallel()
	logger := &fakeLogger{}
	clipboardErr = errors.New("fail")
	CopyItemName(fakeNamed{"foo"}, &models.Environment{}, logger)
	assert.Contains(t, logger.msgs[len(logger.msgs)-1], "failed to copy name")
	clipboardErr = nil
}

func TestCopyTenantID_ClipboardError(t *testing.T) {
	t.Parallel()
	logger := &fakeLogger{}
	clipboardErr = errors.New("fail")
	to := fakeTenancyOverride{"tid"}
	CopyTenantID(to, &models.Environment{}, logger)
	assert.Contains(t, logger.msgs[len(logger.msgs)-1], "failed to copy tenantID")
	clipboardErr = nil
}
