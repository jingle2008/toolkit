package testutil

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	corev1 "k8s.io/api/core/v1"
)

// NewFakeClient returns a fake Kubernetes Clientset pre-loaded with the given objects.
func NewFakeClient(objs ...runtime.Object) *fake.Clientset {
	return fake.NewSimpleClientset(objs...)
}

// TestKubernetesClient is a minimal interface for test fakes.
type TestKubernetesClient interface {
	CoreV1NodesList(ctx context.Context, opts v1.ListOptions) ([]corev1.Node, error)
	CoreV1PodsList(ctx context.Context, namespace string, opts v1.ListOptions) ([]corev1.Pod, error)
}

// NewFakeKubernetesClientAdapter returns a TestKubernetesClient for testing.
func NewFakeKubernetesClientAdapter(clientset *fake.Clientset) TestKubernetesClient {
	return &fakeKubernetesClientAdapter{Clientset: clientset}
}

// fakeKubernetesClientAdapter adapts a *fake.Clientset to the kubernetesClient interface.
type fakeKubernetesClientAdapter struct {
	Clientset *fake.Clientset
}

// CoreV1NodesList implements kubernetesClient by delegating to the fake clientset.
func (f *fakeKubernetesClientAdapter) CoreV1NodesList(ctx context.Context, opts v1.ListOptions) ([]corev1.Node, error) {
	nodeList, err := f.Clientset.CoreV1().Nodes().List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

// CoreV1PodsList implements kubernetesClient by delegating to the fake clientset.
func (f *fakeKubernetesClientAdapter) CoreV1PodsList(ctx context.Context, namespace string, opts v1.ListOptions) ([]corev1.Pod, error) {
	podList, err := f.Clientset.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}
