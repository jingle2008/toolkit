package k8s

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func gpuPod(name, node string, gpus int64, labels, annos map[string]string) *corev1.Pod {
	c := corev1.Container{Name: "main"}
	if gpus > 0 {
		c.Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{gpuProperty: *resource.NewQuantity(gpus, resource.DecimalSI)},
		}
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns1", Labels: labels, Annotations: annos},
		Spec:       corev1.PodSpec{NodeName: node, Containers: []corev1.Container{c}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func TestLoadGPUWorkloadsByNode(t *testing.T) {
	t.Parallel()
	serving := gpuPod("serv", "node-a", 2,
		map[string]string{"tenancy-id": "suffix1", "base-model-name": "gpt", "serving-runtime": "vllm"},
		map[string]string{"ome.io/deploymentMode": "RawDeployment"})
	bare := gpuPod("bare", "node-a", 1, nil, nil)   // GPU pod, no serving labels
	noGPU := gpuPod("nogpu", "node-a", 0, nil, nil) // excluded
	noNode := gpuPod("nonode", "", 4, nil, nil)      // excluded (unscheduled)

	cs := fake.NewSimpleClientset(serving, bare, noGPU, noNode)
	got, err := LoadGPUWorkloadsByNode(context.Background(), cs)
	if err != nil {
		t.Fatal(err)
	}
	if len(got["node-a"]) != 2 {
		t.Fatalf("want 2 workloads on node-a, got %d (%v)", len(got["node-a"]), got)
	}
	for _, w := range got["node-a"] {
		if w.Name == "serv" {
			if w.Model != "gpt" || w.Runtime != "vllm" || w.GPUs != 2 ||
				w.Mode != "RawDeployment" || w.TenantID != "suffix1" || w.Namespace != "ns1" {
				t.Errorf("serv extraction wrong: %+v", w)
			}
		}
		if w.Name == "bare" && (w.Model != "" || w.Runtime != "" || w.GPUs != 1) {
			t.Errorf("bare extraction wrong: %+v", w)
		}
	}
}
