package k8s

import (
	interrors "github.com/jingle2008/toolkit/internal/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewConfig loads a rest.Config from kubeconfig/context.
func NewConfig(kubeconfig, ctx string) (*rest.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: ctx}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, interrors.Wrap("failed to load kubeconfig", err)
	}
	return config, nil
}

func NewClientsetFromRestConfig(config *rest.Config) (*kubernetes.Clientset, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, interrors.Wrap("failed to create k8s client", err)
	}
	return clientset, nil
}

func NewClientsetFromKubeConfig(kubeconfig, ctx string) (*kubernetes.Clientset, error) {
	config, err := NewConfig(kubeconfig, ctx)
	if err != nil {
		return nil, interrors.Wrap("failed to create config from kubeconfig", err)
	}
	return NewClientsetFromRestConfig(config)
}

func NewDynamicClient(config *rest.Config) (*dynamic.DynamicClient, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, interrors.Wrap("failed to create dynamic client", err)
	}
	return dynamicClient, nil
}

func NewDynamicClientFromKubeConfig(kubeconfig, ctx string) (*dynamic.DynamicClient, error) {
	config, err := NewConfig(kubeconfig, ctx)
	if err != nil {
		return nil, interrors.Wrap("failed to create config from kubeconfig", err)
	}
	return NewDynamicClient(config)
}
