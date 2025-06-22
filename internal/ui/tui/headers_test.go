package tui

import (
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestHeaderDefinitions_AllCategoriesPresent(t *testing.T) {
	// Ensure every domain.Category has a header definition
	for _, cat := range []domain.Category{
		domain.Tenant,
		domain.LimitDefinition,
		domain.ConsolePropertyDefinition,
		domain.PropertyDefinition,
		domain.LimitTenancyOverride,
		domain.ConsolePropertyTenancyOverride,
		domain.PropertyTenancyOverride,
		domain.LimitRegionalOverride,
		domain.ConsolePropertyRegionalOverride,
		domain.PropertyRegionalOverride,
		domain.BaseModel,
		domain.ModelArtifact,
		domain.Environment,
		domain.ServiceTenancy,
		domain.GpuPool,
		domain.GpuNode,
		domain.DedicatedAICluster,
	} {
		_, ok := headerDefinitions[cat]
		assert.True(t, ok, "missing header definition for category %v", cat)
	}
}

func TestHeaderDefinitions_RatiosSumToOne(t *testing.T) {
	for cat, headers := range headerDefinitions {
		var sum float64
		for _, h := range headers {
			sum += h.ratio
			assert.NotEmpty(t, h.text, "header text should not be empty")
		}
		assert.InDelta(t, 1.0, sum, 0.01, "ratios for %v do not sum to 1", cat)
	}
}
