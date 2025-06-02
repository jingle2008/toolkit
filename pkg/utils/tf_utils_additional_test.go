package utils

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

func TestGetBaseModel_NilObject(t *testing.T) {
	assert.Panics(t, func() {
		getBaseModel(cty.NilVal, nil, nil)
	}, "getBaseModel should panic on nil cty.Value")
}

func TestGetCapability_NilObject(t *testing.T) {
	assert.Panics(t, func() {
		getCapability(cty.NilVal, nil)
	}, "getCapability should panic on nil cty.Value")
}

func TestLoadGpuPools_Error(t *testing.T) {
	_, err := LoadGpuPools("/no/such/dir", models.Environment{})
	assert.Error(t, err)
}

func TestLoadModelArtifacts_Error(t *testing.T) {
	_, err := LoadModelArtifacts("/no/such/dir", models.Environment{})
	assert.Error(t, err)
}
