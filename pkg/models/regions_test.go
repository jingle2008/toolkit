package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegion_GetCode(t *testing.T) {
	var r Region = "us-phoenix-1"
	assert.Equal(t, "phx", r.GetCode())
	r = "us-ashburn-1"
	assert.Equal(t, "iad", r.GetCode())
	r = "unknown-region"
	assert.Equal(t, "UNKNOWN", r.GetCode())
}

func TestCodeToRegion(t *testing.T) {
	assert.Equal(t, Region("us-phoenix-1"), CodeToRegion("phx"))
	assert.Equal(t, Region("us-ashburn-1"), CodeToRegion("iad"))
	assert.Equal(t, Region(""), CodeToRegion("unknown"))
}
