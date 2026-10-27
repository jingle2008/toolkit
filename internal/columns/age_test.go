package columns

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A row rebuilt from a cached load shows its age as of now, not as of
// the load; a row without a creation time keeps the age it carries.
func TestFreshAge(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "3d", freshAge(time.Now().Add(-3*24*time.Hour-time.Minute), "1d"))
	assert.Equal(t, "1d", freshAge(time.Time{}, "1d"))
	assert.Empty(t, freshAge(time.Time{}, ""))
}
