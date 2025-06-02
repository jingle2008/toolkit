package utils

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockKubernetesClient is a mock implementation of KubernetesClient for testing.
type mockKubernetesClient struct {
	nodes []corev1.Node
	pods  []corev1.Pod
}

func (m *mockKubernetesClient) CoreV1NodesList(ctx context.Context, opts v1.ListOptions) ([]corev1.Node, error) {
	return m.nodes, nil
}

func (m *mockKubernetesClient) CoreV1PodsList(ctx context.Context, namespace string, opts v1.ListOptions) ([]corev1.Pod, error) {
	return m.pods, nil
}

// errorKubernetesClient always returns errors for testing error paths.
type errorKubernetesClient struct{}

func (e *errorKubernetesClient) CoreV1NodesList(ctx context.Context, opts v1.ListOptions) ([]corev1.Node, error) {
	return nil, errors.New("fail nodes")
}

func (e *errorKubernetesClient) CoreV1PodsList(ctx context.Context, namespace string, opts v1.ListOptions) ([]corev1.Pod, error) {
	return nil, errors.New("fail pods")
}

// mockK8sClientAlloc is a mock client for GPU allocation tests.
type mockK8sClientAlloc struct {
	pods []corev1.Pod
}

func (m *mockK8sClientAlloc) CoreV1NodesList(ctx context.Context, opts v1.ListOptions) ([]corev1.Node, error) {
	return nil, nil
}

func (m *mockK8sClientAlloc) CoreV1PodsList(ctx context.Context, namespace string, opts v1.ListOptions) ([]corev1.Pod, error) {
	return m.pods, nil
}
