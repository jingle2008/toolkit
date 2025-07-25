package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubectl/pkg/drain"
)

func TestToggleCordon_TogglesState(t *testing.T) { //nolint:paralleltest // paralleltest is not supported in this package
	ctx := context.Background()
	cs := fake.NewSimpleClientset()
	node := &corev1.Node{
		ObjectMeta: v1.ObjectMeta{Name: "n1"},
		Spec:       corev1.NodeSpec{Unschedulable: false},
	}
	_ = cs.Tracker().Add(node)
	// First toggle: should cordon (set Unschedulable true)
	_, err := toggleCordon(ctx, cs, "n1")
	if err != nil {
		t.Fatalf("toggleCordon failed: %v", err)
	}
	got, _ := cs.CoreV1().Nodes().Get(ctx, "n1", v1.GetOptions{})
	if !got.Spec.Unschedulable {
		t.Error("expected node to be cordoned")
	}
	// Second toggle: should uncordon (set Unschedulable false)
	_, err = toggleCordon(ctx, cs, "n1")
	if err != nil {
		t.Fatalf("toggleCordon failed: %v", err)
	}
	got, _ = cs.CoreV1().Nodes().Get(ctx, "n1", v1.GetOptions{})
	if got.Spec.Unschedulable {
		t.Error("expected node to be uncordoned")
	}
}

func TestToggleCordon_NodeNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cs := fake.NewSimpleClientset()
	_, err := toggleCordon(ctx, cs, "missing-node")
	if err == nil {
		t.Error("expected error for missing node")
	}
}

func TestToggleCordon_CordonOrUncordonError(t *testing.T) { //nolint:paralleltest // paralleltest is not supported in this package
	ctx := context.Background()
	cs := fake.NewSimpleClientset()
	node := &corev1.Node{
		ObjectMeta: v1.ObjectMeta{Name: "n2"},
		Spec:       corev1.NodeSpec{Unschedulable: false},
	}
	_ = cs.Tracker().Add(node)

	orig := runCordonOrUncordon
	defer func() { runCordonOrUncordon = orig }()
	runCordonOrUncordon = func(_ *drain.Helper, _ *corev1.Node, _ bool) error {
		return context.DeadlineExceeded
	}

	_, err := toggleCordon(ctx, cs, "n2")
	if err == nil {
		t.Error("expected error from runCordonOrUncordon")
	}
}

func TestDrainNode_HappyPath(t *testing.T) { //nolint:paralleltest // paralleltest is not supported in this package
	ctx := context.Background()
	cs := fake.NewSimpleClientset()
	node := &corev1.Node{
		ObjectMeta: v1.ObjectMeta{Name: "dn3"},
		Spec:       corev1.NodeSpec{Unschedulable: false},
	}
	_ = cs.Tracker().Add(node)

	orig := runNodeDrain
	defer func() { runNodeDrain = orig }()
	runNodeDrain = func(_ *drain.Helper, _ string) error {
		return nil
	}

	err := drainNode(ctx, cs, "dn3")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDrainNode_Error(t *testing.T) { //nolint:paralleltest // paralleltest is not supported in this package
	ctx := context.Background()
	cs := fake.NewSimpleClientset()
	node := &corev1.Node{
		ObjectMeta: v1.ObjectMeta{Name: "n4"},
		Spec:       corev1.NodeSpec{Unschedulable: false},
	}
	_ = cs.Tracker().Add(node)

	orig := runNodeDrain
	defer func() { runNodeDrain = orig }()
	runNodeDrain = func(_ *drain.Helper, _ string) error {
		return context.Canceled
	}

	err := drainNode(ctx, cs, "n4")
	if err == nil {
		t.Error("expected error from runNodeDrain")
	}
}
