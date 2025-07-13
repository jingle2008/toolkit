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
CordonNode uses kubectl's drain.Helper to cordon a node.
*/
func CordonNode(ctx context.Context, kubeconfig, contextName, nodeName string) error {
	clientset, err := NewClientsetFromKubeConfig(kubeconfig, contextName)
	if err != nil {
		return err
	}
	return setCordon(ctx, clientset, nodeName, true)
}

/*
UncordonNode uses kubectl's drain.Helper to uncordon a node.
*/
func UncordonNode(ctx context.Context, kubeconfig, contextName, nodeName string) error {
	clientset, err := NewClientsetFromKubeConfig(kubeconfig, contextName)
	if err != nil {
		return err
	}
	return setCordon(ctx, clientset, nodeName, false)
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

func setCordon(ctx context.Context, clientset kubernetes.Interface, nodeName string, desired bool) error {
	helper := &drain.Helper{
		Ctx:    ctx,
		Client: clientset,
	}

	// Get the current node
	node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, v1.GetOptions{})
	if err != nil {
		return err
	}

	// Apply the desired cordon state
	return runCordonOrUncordon(helper, node, desired)
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
