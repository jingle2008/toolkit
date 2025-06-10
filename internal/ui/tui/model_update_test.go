package tui

import (
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/internal/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestLoadRequest_Run(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	lr := loadRequest{
		category: domain.BaseModel,
		model:    m,
	}
	cmd := lr.Run()
	assert.NotNil(t, cmd)
}
