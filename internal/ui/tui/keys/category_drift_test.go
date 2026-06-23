package keys

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
)

// noContextKeys are categories that intentionally have no per-category key
// bindings. A new category must be added here OR to catContext — never
// silently neither. Keep in sync with catContext (registry.go).
var noContextKeys = map[domain.Category]struct{}{
	domain.LimitDefinition: {},
	domain.ModelArtifact:   {},
	domain.Alias:           {},
}

func TestCatContext_EveryCategoryAccountedFor(t *testing.T) {
	t.Parallel()
	for _, c := range domain.Categories {
		_, hasKeys := catContext[c]
		_, excluded := noContextKeys[c]
		assert.Truef(t, hasKeys != excluded,
			"%s must be in exactly one of catContext / noContextKeys (hasKeys=%v excluded=%v)",
			c, hasKeys, excluded)
	}
}
