package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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

// TestLoadGPUWorkloadsByNode_Paginates verifies the loader follows the
// Continue token and accumulates workloads across pages. The fake
// clientset ignores Limit/Continue, so a reactor simulates a paged
// response: page 1 carries a Continue token, page 2 clears it.
func TestLoadGPUWorkloadsByNode_Paginates(t *testing.T) {
	t.Parallel()

	page1 := &corev1.PodList{
		ListMeta: metav1.ListMeta{Continue: "next-page-token"},
		Items:    []corev1.Pod{*gpuPod("p1", "node-a", 1, nil, nil)},
	}
	page2 := &corev1.PodList{
		Items: []corev1.Pod{*gpuPod("p2", "node-b", 1, nil, nil)},
	}

	cs := fake.NewSimpleClientset()
	calls := 0
	cs.PrependReactor("list", "pods", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		calls++
		if calls == 1 {
			return true, page1, nil
		}
		return true, page2, nil
	})

	got, err := LoadGPUWorkloadsByNode(context.Background(), cs)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 paged List calls, got %d", calls)
	}
	if len(got["node-a"]) != 1 || len(got["node-b"]) != 1 {
		t.Fatalf("expected one workload on each of node-a and node-b, got %v", got)
	}
}
