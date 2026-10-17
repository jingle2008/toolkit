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
SetCordon brings the node to the requested cordon state. Idempotent:
when the node is already in `want`, no API call is made and changed
is false. Otherwise the node is updated and changed is true. Every
surface (TUI, CLI, MCP) cordons through it, naming the state it wants
rather than flipping whatever the live state is.
*/
func SetCordon(ctx context.Context, kubeconfig, contextName, nodeName string, want bool) (changed bool, err error) {
	clientset, err := NewClientsetFromKubeConfig(kubeconfig, contextName)
	if err != nil {
		return false, err
	}
	return setCordon(ctx, clientset, nodeName, want)
}

type logWriter struct{ logger logging.Logger }

func (w logWriter) Write(p []byte) (int, error) {
	w.logger.Infow("kubectl-drain", "msg", strings.TrimSpace(string(p)))
	return len(p), nil
}

var runCordonOrUncordon = drain.RunCordonOrUncordon

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
