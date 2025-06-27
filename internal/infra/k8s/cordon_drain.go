package k8s

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/drain"
)

func clientsetFromKubeconfig(kubeconfig, contextName string) (*kubernetes.Clientset, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}
	return clientset, nil
}

/*
ToggleCordon uses kubectl's drain.Helper to cordon or uncordon a node.
Returns true if node is cordoned after the call, false if uncordoned.
*/
func ToggleCordon(ctx context.Context, kubeconfig, contextName, nodeName string) error {
	clientset, err := clientsetFromKubeconfig(kubeconfig, contextName)
	if err != nil {
		return err
	}
	return toggleCordon(ctx, clientset, nodeName)
}

/*
DrainNode uses kubectl's drain.Helper to cordon and drain a node.
*/
func DrainNode(ctx context.Context, kubeconfig, contextName, nodeName string) error {
	clientset, err := clientsetFromKubeconfig(kubeconfig, contextName)
	if err != nil {
		return err
	}
	return drainNode(ctx, clientset, nodeName)
}

func toggleCordon(ctx context.Context, clientset kubernetes.Interface, nodeName string) error {
	helper := &drain.Helper{
		Ctx:    ctx,
		Client: clientset,
	}

	// Get the current node
	node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, v1.GetOptions{})
	if err != nil {
		return err
	}

	// Determine new cordon state (toggle current value)
	cordonState := !node.Spec.Unschedulable

	// Apply the new state
	return drain.RunCordonOrUncordon(helper, node, cordonState)
}

func drainNode(ctx context.Context, clientset kubernetes.Interface, nodeName string) error {
	helper := &drain.Helper{
		Ctx:                 ctx,
		Client:              clientset,
		IgnoreAllDaemonSets: true,
		DeleteEmptyDirData:  true,
		GracePeriodSeconds:  -1, // Use pod's termination grace period
	}
	return drain.RunNodeDrain(helper, nodeName)
}
