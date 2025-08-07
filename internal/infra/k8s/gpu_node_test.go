package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

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
			got := FormatAge(tt.dur)
			if got != tt.expected {
				t.Errorf("FormatAge(%v) = %q, want %q", tt.dur, got, tt.expected)
			}
		})
	}
}

func TestGetNodeIssues(t *testing.T) {
	t.Parallel()
	conds := []corev1.NodeCondition{
		{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue, Message: "memory pressure!"},
		{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse, Message: "disk ok"},
		{Type: corev1.NodePIDPressure, Status: corev1.ConditionTrue, Message: "pid pressure!"},
		{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionTrue, Message: "network unavailable!"},
		{Type: nodeCondGpuBus, Status: corev1.ConditionTrue, Message: "gpu bus error!"},
		{Type: nodeCondGpuCount, Status: corev1.ConditionTrue, Message: "gpu count mismatch!"},
		{Type: corev1.NodeReady, Status: corev1.ConditionTrue, Message: "node is ready"},
	}
	want := []string{
		"memory pressure!",
		"pid pressure!",
		"network unavailable!",
		"gpu bus error!",
		"gpu count mismatch!",
	}
	got := getNodeIssues(conds)
	assert.ElementsMatch(t, want, got)
}

func TestGetPodReason(t *testing.T) {
	t.Parallel()
	// 1. Status.Reason present
	pod1 := &corev1.Pod{
		Status: corev1.PodStatus{
			Reason: "Evicted",
		},
	}
	assert.Equal(t, "Evicted", getPodReason(pod1))

	// 2. Waiting state in first container
	pod2 := &corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"},
					},
				},
			},
		},
	}
	assert.Equal(t, "ImagePullBackOff", getPodReason(pod2))

	// 3. Terminated state in first container
	pod3 := &corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled"},
					},
				},
			},
		},
	}
	assert.Equal(t, "OOMKilled", getPodReason(pod3))

	// 4. No reason anywhere
	pod4 := &corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{},
				},
			},
		},
	}
	assert.Equal(t, "unknown", getPodReason(pod4))

	// 5. No container statuses at all
	pod5 := &corev1.Pod{}
	assert.Equal(t, "unknown", getPodReason(pod5))
}
