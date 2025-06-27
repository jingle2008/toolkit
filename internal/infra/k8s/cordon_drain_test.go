package k8s

import (
	"context"
	"testing"

	"github.com/jingle2008/toolkit/internal/testutil"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClientsetFromKubeconfig_BadPath(t *testing.T) {
	t.Parallel()
	_, err := clientsetFromKubeconfig("/not/a/real/path", "")
	if err == nil {
		t.Error("expected error for bad kubeconfig path")
	}
}

func TestToggleCordon_TogglesState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cs := testutil.NewFakeClient()
	node := &corev1.Node{
		ObjectMeta: v1.ObjectMeta{Name: "n1"},
		Spec:       corev1.NodeSpec{Unschedulable: false},
	}
	_ = cs.Tracker().Add(node)
	// First toggle: should cordon (set Unschedulable true)
	err := toggleCordon(ctx, cs, "n1")
	if err != nil {
		t.Fatalf("toggleCordon failed: %v", err)
	}
	got, _ := cs.CoreV1().Nodes().Get(ctx, "n1", v1.GetOptions{})
	if !got.Spec.Unschedulable {
		t.Error("expected node to be cordoned")
	}
	// Second toggle: should uncordon (set Unschedulable false)
	err = toggleCordon(ctx, cs, "n1")
	if err != nil {
		t.Fatalf("toggleCordon failed: %v", err)
	}
	got, _ = cs.CoreV1().Nodes().Get(ctx, "n1", v1.GetOptions{})
	if got.Spec.Unschedulable {
		t.Error("expected node to be uncordoned")
	}
}
