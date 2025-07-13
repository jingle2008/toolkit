package tui

import (
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestGetTableRows_AliasCategory(t *testing.T) {
	t.Parallel()
	logger := logging.NewNoOpLogger()
	dataset := &models.Dataset{}
	rows := getTableRows(logger, dataset, domain.Alias, nil, "", "", true)
	assert.Equal(t, len(domain.Categories), len(rows), "should return one row per category")

	// Find GpuNode row
	found := false
	for _, row := range rows {
		if len(row) > 0 && row[0] == "GpuNode" {
			found = true
			break
		}
	}
	assert.True(t, found, "GpuNode row should be present")

	// Filtering
	filtered := getTableRows(logger, dataset, domain.Alias, nil, "tenant", "", true)
	assert.Equal(t, 1, len(filtered), "filter 'tenant' should return exactly one row")
	assert.Equal(t, "Tenant", filtered[0][0])
}
