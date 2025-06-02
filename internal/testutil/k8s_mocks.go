// Package testutil provides mock Kubernetes clients for testing.
package testutil

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ErrorKubernetesClient always returns errors for testing error paths.
type ErrorKubernetesClient struct{}

/*
CoreV1NodesList always returns an error for testing error paths.
*/
func (e *ErrorKubernetesClient) CoreV1NodesList(_ context.Context, _ v1.ListOptions) ([]corev1.Node, error) {
	return nil, errors.New("fail nodes")
}

/*
CoreV1PodsList always returns an error for testing error paths.
*/
func (e *ErrorKubernetesClient) CoreV1PodsList(_ context.Context, _ string, _ v1.ListOptions) ([]corev1.Pod, error) {
	return nil, errors.New("fail pods")
}

// MockK8sClientAlloc is a mock client for GPU allocation tests.
type MockK8sClientAlloc struct {
	Pods []corev1.Pod
}

/*
CoreV1NodesList returns nil for testing.
*/
func (m *MockK8sClientAlloc) CoreV1NodesList(_ context.Context, _ v1.ListOptions) ([]corev1.Node, error) {
	return nil, nil
}

/*
CoreV1PodsList returns the mock pod list for testing.
*/
func (m *MockK8sClientAlloc) CoreV1PodsList(_ context.Context, _ string, _ v1.ListOptions) ([]corev1.Pod, error) {
	return m.Pods, nil
}

// MockKubernetesClient is a mock implementation of KubernetesClient for testing.
type MockKubernetesClient struct {
	Nodes []corev1.Node
	Pods  []corev1.Pod
}

/*
CoreV1NodesList returns the mock node list for testing.
*/
func (m *MockKubernetesClient) CoreV1NodesList(_ context.Context, _ v1.ListOptions) ([]corev1.Node, error) {
	return m.Nodes, nil
}

/*
CoreV1PodsList returns the mock pod list for testing.
*/
func (m *MockKubernetesClient) CoreV1PodsList(_ context.Context, _ string, _ v1.ListOptions) ([]corev1.Pod, error) {
	return m.Pods, nil
}
