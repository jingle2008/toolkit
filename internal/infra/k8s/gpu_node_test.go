package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestIsNodeHealthy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		conds []corev1.NodeCondition
		want  bool
	}{
		{
			name: "healthy GPU",
			conds: []corev1.NodeCondition{
				{Type: nodeCondGpuUnhealthy, Status: corev1.ConditionFalse},
			},
			want: true,
		},
		{
			name: "unhealthy GPU",
			conds: []corev1.NodeCondition{
				{Type: nodeCondGpuUnhealthy, Status: corev1.ConditionTrue},
			},
			want: false,
		},
		{
			name: "no GPU condition",
			conds: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
			want: false,
		},
		{
			name:  "empty conditions",
			conds: nil,
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isNodeHealthy(tt.conds))
		})
	}
}

func TestIsNodeReady(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		conds []corev1.NodeCondition
		want  bool
	}{
		{
			name: "ready node",
			conds: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
			want: true,
		},
		{
			name: "not ready node",
			conds: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
			},
			want: false,
		},
		{
			name: "no ready condition",
			conds: []corev1.NodeCondition{
				{Type: nodeCondGpuUnhealthy, Status: corev1.ConditionFalse},
			},
			want: false,
		},
		{
			name:  "empty conditions",
			conds: nil,
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isNodeReady(tt.conds))
		})
	}
}

func TestCalculatePodGPUs(t *testing.T) {
	t.Parallel()
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceName(gpuProperty): *resource.NewQuantity(3, resource.DecimalSI),
					},
				}},
			},
		},
	}
	assert.Equal(t, int64(3), calculatePodGPUs(&pod))
}

func TestFormatAge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		dur      time.Duration
		expected string
	}{
		{time.Second * 30, "30s"},
		{time.Minute, "1m"},
		{time.Minute*59 + time.Second*59, "59m"},
		{time.Hour, "1h"},
		{time.Hour*23 + time.Minute*59, "23h"},
		{time.Hour * 48, "2d"},
		{time.Hour*72 + time.Minute*1, "3d"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()
			got := formatAge(tt.dur)
			if got != tt.expected {
				t.Errorf("formatAge(%v) = %q, want %q", tt.dur, got, tt.expected)
			}
		})
	}
}
