package domain

import (
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
)

// FilterEnvironments returns a filtered slice of Environment matching the provided filter string.
func FilterEnvironments(envs []models.Environment, filter string) []models.Environment {
	results := make([]models.Environment, 0, len(envs))

	utils.FilterSlice(envs, nil, filter, func(_ int, val models.Environment) bool {
		results = append(results, val)
		return true
	})

	return results
}
