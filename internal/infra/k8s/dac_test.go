package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTenantIDFromLabels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		labels map[string]any
		want   string
	}{
		{
			name:   "string tenancy-id",
			labels: map[string]any{"tenancy-id": "tid"},
			want:   "tid",
		},
		{
			name:   "missing tenancy-id",
			labels: map[string]any{},
			want:   "UNKNOWN_TENANCY",
		},
		{
			name:   "non-string tenancy-id",
			labels: map[string]any{"tenancy-id": 123},
			want:   "UNKNOWN_TENANCY",
		},
		{
			name:   "nil labels",
			labels: nil,
			want:   "UNKNOWN_TENANCY",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tenantIDFromLabels(tt.labels))
		})
	}
}
