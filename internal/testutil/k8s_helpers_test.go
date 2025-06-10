package testutil

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewFakeClientAndAdapterLists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Seed objects
	n1 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}
	p1 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "ns1"}}
	p2 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-b", Namespace: "ns2"}}

	client := NewFakeClient(n1, p1, p2)
	adapter := &fakeKubernetesClientAdapter{Clientset: client}

	// Nodes
	nodes, err := adapter.CoreV1NodesList(ctx, metav1.ListOptions{})
	RequireNoError(t, err)
	RequireEqual(t, 1, len(nodes))
	RequireEqual(t, "node-a", nodes[0].Name)

	// Pods filtered by namespace
	podsNs1, err := adapter.CoreV1PodsList(ctx, "ns1", metav1.ListOptions{})
	RequireNoError(t, err)
	RequireEqual(t, 1, len(podsNs1))
	RequireEqual(t, "pod-a", podsNs1[0].Name)

	podsNs2, err := adapter.CoreV1PodsList(ctx, "ns2", metav1.ListOptions{})
	RequireNoError(t, err)
	RequireEqual(t, 1, len(podsNs2))
	RequireEqual(t, "pod-b", podsNs2[0].Name)

	// Empty namespace
	empty, err := adapter.CoreV1PodsList(ctx, "does-not-exist", metav1.ListOptions{})
	RequireNoError(t, err)
	RequireEqual(t, 0, len(empty))
}

func TestAdapterNilClientPanics(t *testing.T) {
	t.Parallel()
	adapter := &fakeKubernetesClientAdapter{Clientset: nil}
	AssertPanic(t, func() {
		_, _ = adapter.CoreV1NodesList(context.Background(), metav1.ListOptions{})
	})
}
