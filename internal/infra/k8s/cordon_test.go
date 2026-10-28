package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubectl/pkg/drain"
)

func TestSetCordon_CordonOrUncordonError(t *testing.T) { //nolint:paralleltest // shared global runCordonOrUncordon
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

	if _, err := setCordon(ctx, cs, "n2", true); err == nil {
		t.Error("expected error from runCordonOrUncordon")
	}
}

func TestSetCordon_NoOpWhenAlreadyCordoned(t *testing.T) { //nolint:paralleltest // shared global runCordonOrUncordon
	ctx := context.Background()
	cs := fake.NewSimpleClientset()
	node := &corev1.Node{
		ObjectMeta: v1.ObjectMeta{Name: "n-already-cordoned"},
		Spec:       corev1.NodeSpec{Unschedulable: true},
	}
	_ = cs.Tracker().Add(node)

	called := false
	orig := runCordonOrUncordon
	defer func() { runCordonOrUncordon = orig }()
	runCordonOrUncordon = func(_ *drain.Helper, _ *corev1.Node, _ bool) error {
		called = true
		return nil
	}

	changed, err := setCordon(ctx, cs, "n-already-cordoned", true)
	if err != nil {
		t.Fatalf("setCordon: %v", err)
	}
	if changed {
		t.Error("expected changed=false when node already in target state")
	}
	if called {
		t.Error("must not call runCordonOrUncordon when already in target state")
	}
}

func TestSetCordon_AppliesWhenDifferent(t *testing.T) { //nolint:paralleltest // shared global runCordonOrUncordon
	ctx := context.Background()
	cs := fake.NewSimpleClientset()
	node := &corev1.Node{
		ObjectMeta: v1.ObjectMeta{Name: "n-uncordoned"},
		Spec:       corev1.NodeSpec{Unschedulable: false},
	}
	_ = cs.Tracker().Add(node)

	var gotWant bool
	orig := runCordonOrUncordon
	defer func() { runCordonOrUncordon = orig }()
	runCordonOrUncordon = func(_ *drain.Helper, _ *corev1.Node, want bool) error {
		gotWant = want
		return nil
	}

	changed, err := setCordon(ctx, cs, "n-uncordoned", true)
	if err != nil {
		t.Fatalf("setCordon: %v", err)
	}
	if !changed {
		t.Error("expected changed=true")
	}
	if !gotWant {
		t.Error("expected runCordonOrUncordon called with want=true")
	}
}

func TestSetCordon_NodeNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cs := fake.NewSimpleClientset()
	_, err := setCordon(ctx, cs, "missing", true)
	if err == nil {
		t.Error("expected error for missing node")
	}
}
