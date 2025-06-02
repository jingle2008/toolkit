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

// FakeKubernetesClientAdapter adapts a *fake.Clientset to the KubernetesClient interface.
type FakeKubernetesClientAdapter struct {
	Clientset *fake.Clientset
}

// CoreV1NodesList implements KubernetesClient by delegating to the fake clientset.
func (f *FakeKubernetesClientAdapter) CoreV1NodesList(ctx context.Context, opts v1.ListOptions) ([]corev1.Node, error) {
	nodeList, err := f.Clientset.CoreV1().Nodes().List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

// CoreV1PodsList implements KubernetesClient by delegating to the fake clientset.
func (f *FakeKubernetesClientAdapter) CoreV1PodsList(ctx context.Context, namespace string, opts v1.ListOptions) ([]corev1.Pod, error) {
	podList, err := f.Clientset.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}
