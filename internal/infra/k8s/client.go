/*
Package k8s provides Kubernetes client utilities for loading configs and creating typed and dynamic clients.
*/
package k8s

import (
	"errors"
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
	// Apply sensible defaults for client throttling; allow override if already set.
	if config.QPS == 0 {
		config.QPS = 20
	}
	if config.Burst == 0 {
		config.Burst = 40
	}
	// Identify this client in user agent.
	rest.AddUserAgent(config, "toolkit")
	return config, nil
}

/*
NewClientsetFromRestConfig creates a new Kubernetes clientset from the given rest.Config.
Returns an error if the config is nil or client creation fails.
*/
func NewClientsetFromRestConfig(config *rest.Config) (kubernetes.Interface, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}
	return clientset, nil
}

/*
NewClientsetFromKubeConfig creates a new Kubernetes clientset from a kubeconfig file and context.
Returns an error if config loading or client creation fails.
*/
func NewClientsetFromKubeConfig(kubeconfig, ctx string) (kubernetes.Interface, error) {
	config, err := NewConfig(kubeconfig, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create config from kubeconfig: %w", err)
	}
	return NewClientsetFromRestConfig(config)
}

/*
NewDynamicClient creates a new dynamic Kubernetes client from the given rest.Config.
Returns an error if the config is nil or client creation fails.
*/
func NewDynamicClient(config *rest.Config) (dynamic.Interface, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	return dynamicClient, nil
}

/*
NewDynamicClientFromKubeConfig creates a new dynamic Kubernetes client from a kubeconfig file and context.
Returns an error if config loading or client creation fails.
*/
func NewDynamicClientFromKubeConfig(kubeconfig, ctx string) (dynamic.Interface, error) {
	config, err := NewConfig(kubeconfig, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create config from kubeconfig: %w", err)
	}
	return NewDynamicClient(config)
}
