package view

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// stripANSI removes ANSI escape codes from a string.
func stripANSI(input string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(input, "")
}

func TestProductionRenderer_RenderJSON_Success(t *testing.T) {
	t.Parallel()
	r := ProductionRenderer{}
	data := map[string]interface{}{
		"foo": "bar",
		"num": 42,
	}
	out, err := r.RenderJSON(data, 40)
	assert.NoError(t, err)
	out = stripANSI(out)
	assert.Contains(t, out, "foo")
	assert.Contains(t, out, "bar")
	assert.Contains(t, out, "num")
}

// glamour.NewTermRenderer does not error for negative width, so we do not test error path here.
