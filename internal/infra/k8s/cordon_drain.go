package k8s

import (
	"context"
	"strings"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"
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
