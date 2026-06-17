package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImportedModel_OwnerState(t *testing.T) {
	t.Parallel()
	var nilOwner ImportedModel
	assert.Equal(t, "", nilOwner.OwnerState(), "nil owner: want empty string")

	internal := ImportedModel{Owner: &Tenant{IsInternal: true}}
	assert.Equal(t, "true", internal.OwnerState(), "internal owner: want \"true\"")

	external := ImportedModel{Owner: &Tenant{IsInternal: false}}
	assert.Equal(t, "false", external.OwnerState(), "external owner: want \"false\"")
}
