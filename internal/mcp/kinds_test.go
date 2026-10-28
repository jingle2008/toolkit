package mcp

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// kindNames, which the error and the tool descriptions quote, and the
// kind field's schema text name exactly overrideKinds' kinds.
func TestOverrideKinds_NamesMatch(t *testing.T) {
	t.Parallel()
	names := strings.Split(kindNames, ", ")
	keys := make([]string, 0, len(overrideKinds))
	for k := range overrideKinds {
		keys = append(keys, k)
	}
	slices.Sort(names)
	slices.Sort(keys)
	assert.Equal(t, keys, names)

	f, _ := reflect.TypeFor[kindInput]().FieldByName("Kind")
	assert.Contains(t, f.Tag.Get("jsonschema"), kindNames)
}

// A kind resolves case-insensitively; an unknown one names the kinds.
func TestKindInput_Categories(t *testing.T) {
	t.Parallel()
	c, err := kindInput{Kind: " Console_Property "}.categories()
	assert.NoError(t, err)
	assert.Equal(t, overrideKinds["console_property"], c)

	_, err = kindInput{Kind: "quota"}.categories()
	assert.EqualError(t, err, `unknown kind "quota" (expected: `+kindNames+`)`)
}
