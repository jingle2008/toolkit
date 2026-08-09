package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
)

func TestHasIntHeader(t *testing.T) {
	t.Parallel()
	set := map[string]struct{}{"Size": {}, "GPUs": {}}

	assert.True(t, set != nil && hasIntHeader(set, "Size"))
	// Matching is case-insensitive so header casing changes don't silently
	// switch a column back to lexical sorting.
	assert.True(t, hasIntHeader(set, "size"))
	assert.True(t, hasIntHeader(set, "GPUS"))
	assert.False(t, hasIntHeader(set, "Name"))
	assert.False(t, hasIntHeader(map[string]struct{}{}, "Size"))
}

func TestParsePercent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{in: "37%", want: 37},
		{in: "0%", want: 0},
		{in: "100%", want: 100},
		{in: " 50", want: 50, /* suffix is optional */},
		// TrimSuffix runs before TrimSpace, so a trailing space defeats the
		// "%" strip. Harmless in practice — cells are formatted "%.0f%%"
		// with no padding — but pinned so the ordering isn't changed blindly.
		{in: " 50% ", wantErr: true},
		// .5 rounds up, matching the "%.0f" the model formats with.
		{in: "66.5%", want: 67},
		{in: "66.4%", want: 66},
		{in: "", wantErr: true},
		{in: "n/a", wantErr: true},
		{in: "%", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()
			got, err := parsePercent(c.in)
			if c.wantErr {
				require.Error(t, err)
				assert.Zero(t, got, "error case should return 0")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestItemKeyFrom_EmptyRow(t *testing.T) {
	t.Parallel()
	// A nil key is what findItem treats as "nothing selected"; returning
	// row[0] on an empty row would panic instead.
	assert.Nil(t, itemKeyFrom(domain.Tenant, table.Row{}))
	assert.Nil(t, itemKeyFrom(domain.GPUNode, nil))
}

func TestItemKeyFrom_FlatCategoryUsesFirstColumn(t *testing.T) {
	t.Parallel()
	got := itemKeyFrom(domain.Tenant, table.Row{"tenant-a", "extra"})
	assert.Equal(t, "tenant-a", got)
}

func TestMetadataPath_FallbackWhenLoaderLacksGetter(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	// fakeLoader has no MetadataPath method, so the display falls back to
	// a placeholder rather than an empty string.
	assert.Equal(t, "metadata file", m.metadataPath())
}

// metadataPathLoader is a loader that also exposes MetadataPath, matching
// the optional getter production.Client provides.
type metadataPathLoader struct {
	fakeLoader
	path string
}

func (l metadataPathLoader) MetadataPath() string { return l.path }

func TestMetadataPath_UsesLoaderGetter(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.loader = metadataPathLoader{path: "/etc/toolkit/metadata.yaml"}
	assert.Equal(t, "/etc/toolkit/metadata.yaml", m.metadataPath())
}
