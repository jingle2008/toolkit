package production

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/infra/loader"
)

func TestClient_ImplementsWatcher(t *testing.T) {
	t.Parallel()
	assert.Implements(t, (*loader.Watcher)(nil), &Client{})
}
