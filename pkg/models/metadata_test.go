package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadata_Validate(t *testing.T) {
	t.Parallel()
	name := func(s string) *string { return &s }

	require.NoError(t, (&Metadata{Tenants: []TenantMetadata{
		{ID: "a", Name: name("Acme")},
		{ID: "b", Name: name("Beta")},
		{ID: "c"},
		{ID: "d", Name: name("")}, // unnamed entries don't collide
	}}).Validate())

	err := (&Metadata{Tenants: []TenantMetadata{
		{ID: "a", Name: name("Acme")},
		{ID: "b", Name: name("Acme")},
		{ID: "a", Name: name("Other")},
	}}).Validate()
	require.Error(t, err)
	assert.ErrorContains(t, err, `tenant name "Acme" is used by both a and b`)
	assert.ErrorContains(t, err, "tenant id a is listed more than once")
}
