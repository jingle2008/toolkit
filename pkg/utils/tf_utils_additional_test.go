package utils

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
)

func TestGetLocalAttributesDI_ListFilesError(t *testing.T) {
	_, err := getLocalAttributesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return nil, assert.AnError },
		func(string, hclsyntax.Attributes) error { return nil },
	)
	assert.Error(t, err)
}

func TestGetLocalAttributesDI_UpdateLocalAttributesError(t *testing.T) {
	files := []string{"a.tf", "b.tf"}
	_, err := getLocalAttributesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return files, nil },
		func(string, hclsyntax.Attributes) error { return assert.AnError },
	)
	assert.Error(t, err)
}

func TestGetLocalAttributesDI_EmptyFiles(t *testing.T) {
	out, err := getLocalAttributesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return []string{}, nil },
		func(string, hclsyntax.Attributes) error { return nil },
	)
	assert.NoError(t, err)
	assert.NotNil(t, out)
	assert.Len(t, out, 0)
}

func TestGetLocalAttributesDI_Success(t *testing.T) {
	files := []string{"a.tf"}
	called := false
	_, err := getLocalAttributesDI(
		"irrelevant",
		func(string, string) ([]string, error) { return files, nil },
		func(string, hclsyntax.Attributes) error { called = true; return nil },
	)
	assert.NoError(t, err)
	assert.True(t, called)
}
