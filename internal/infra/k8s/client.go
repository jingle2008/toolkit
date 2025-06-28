package k8s

import (
	"fmt"

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
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}
	return config, nil
}

func NewClientsetFromRestConfig(config *rest.Config) (*kubernetes.Clientset, error) {
	if config == nil {
		return nil, fmt.Errorf("nil config: %w", fmt.Errorf("config is nil"))
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}
	return clientset, nil
}

func NewClientsetFromKubeConfig(kubeconfig, ctx string) (*kubernetes.Clientset, error) {
	config, err := NewConfig(kubeconfig, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create config from kubeconfig: %w", err)
	}
	return NewClientsetFromRestConfig(config)
}

func NewDynamicClient(config *rest.Config) (*dynamic.DynamicClient, error) {
	if config == nil {
		return nil, fmt.Errorf("nil config: %w", fmt.Errorf("config is nil"))
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	return dynamicClient, nil
}

func NewDynamicClientFromKubeConfig(kubeconfig, ctx string) (*dynamic.DynamicClient, error) {
	config, err := NewConfig(kubeconfig, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create config from kubeconfig: %w", err)
	}
	return NewDynamicClient(config)
}
