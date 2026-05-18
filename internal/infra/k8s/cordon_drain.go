package k8s

import (
	"context"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
)

/*
ToggleCordon uses kubectl's drain.Helper to cordon or uncordon a node.
Returns true if node is cordoned after the call, false if uncordoned.
*/
func ToggleCordon(ctx context.Context, kubeconfig, contextName, nodeName string) (bool, error) {
	clientset, err := NewClientsetFromKubeConfig(kubeconfig, contextName)
	if err != nil {
		return false, err
	}
	return toggleCordon(ctx, clientset, nodeName)
}

/*
SetCordon brings the node to the requested cordon state. Idempotent:
when the node is already in `want`, no API call is made and changed
is false. Otherwise the node is updated and changed is true. Use this
instead of ToggleCordon for repeatable scripted / agent calls.
*/
func SetCordon(ctx context.Context, kubeconfig, contextName, nodeName string, want bool) (changed bool, err error) {
	clientset, err := NewClientsetFromKubeConfig(kubeconfig, contextName)
	if err != nil {
		return false, err
	}
	return setCordon(ctx, clientset, nodeName, want)
}

/*
DrainNode uses kubectl's drain.Helper to cordon and drain a node.
*/
func DrainNode(ctx context.Context, kubeconfig, contextName, nodeName string) error {
	clientset, err := NewClientsetFromKubeConfig(kubeconfig, contextName)
	if err != nil {
		return err
	}
	return drainNode(ctx, clientset, nodeName)
}

type logWriter struct{ logger logging.Logger }

func (w logWriter) Write(p []byte) (int, error) {
	w.logger.Infow("kubectl-drain", "msg", strings.TrimSpace(string(p)))
	return len(p), nil
}

var (
	runCordonOrUncordon = drain.RunCordonOrUncordon
	runNodeDrain        = drain.RunNodeDrain
)

func toggleCordon(ctx context.Context, clientset kubernetes.Interface, nodeName string) (bool, error) {
	helper := &drain.Helper{
		Ctx:    ctx,
		Client: clientset,
	}

	// Get the current node
	node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, v1.GetOptions{})
	if err != nil {
		return false, err
	}

	// Determine new cordon state (toggle current value)
	cordonState := !node.Spec.Unschedulable

	// Apply the new state
	return cordonState, runCordonOrUncordon(helper, node, cordonState)
}

// setCordon is the testable inner of SetCordon. Returns (changed, err)
// where changed is false when the node was already in the requested
// state.
func setCordon(ctx context.Context, clientset kubernetes.Interface, nodeName string, want bool) (bool, error) {
	node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, v1.GetOptions{})
	if err != nil {
		return false, err
	}
	if node.Spec.Unschedulable == want {
		return false, nil
	}
	helper := &drain.Helper{Ctx: ctx, Client: clientset}
	return true, runCordonOrUncordon(helper, node, want)
}

func drainNode(ctx context.Context, clientset kubernetes.Interface, nodeName string) error {
	logger := logging.FromContext(ctx)
	helper := &drain.Helper{
		Ctx:                 ctx,
		Client:              clientset,
		Out:                 logWriter{logger},
		ErrOut:              logWriter{logger},
		IgnoreAllDaemonSets: true,
		DeleteEmptyDirData:  true,
		GracePeriodSeconds:  -1, // Use pod's termination grace period
	}
	return runNodeDrain(helper, nodeName)
}
