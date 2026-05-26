package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegion_GetCode(t *testing.T) {
	t.Parallel()
	var r Region = "us-phoenix-1"
	assert.Equal(t, "phx", r.Code())
	r = "us-ashburn-1"
	assert.Equal(t, "iad", r.Code())

	// Mapped explicitly after SDK v65.114 added Newark.
	r = "us-newark-1"
	assert.Equal(t, "pgc", r.Code())

	// Sovereign-cloud / unmapped region: falls back to the city
	// segment instead of literal "UNKNOWN" so the table stays
	// identifiable until an explicit mapping is added.
	r = "ap-westtokyo-1"
	assert.Equal(t, "westtokyo", r.Code())

	// Input doesn't look like a region identifier — still UNKNOWN.
	r = "unknown-region"
	assert.Equal(t, "UNKNOWN", r.Code())
}

func TestCodeToRegion(t *testing.T) {
	t.Parallel()
	assert.Equal(t, Region("us-phoenix-1"), CodeToRegion("phx"))
	assert.Equal(t, Region("us-ashburn-1"), CodeToRegion("iad"))
	assert.Equal(t, Region(""), CodeToRegion("unknown"))
}
