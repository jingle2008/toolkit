/*
Package k8s provides Kubernetes client utilities for loading configs and creating typed and dynamic clients.
*/
package k8s

import (
	"errors"
	"fmt"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// RequestTimeout caps every client-go call NewConfig's configs make, so a
// broken or unreachable cluster fails fast instead of hanging on a TCP
// dial / TLS handshake. Without it, an interactive run sits on the
// spinner until SIGINT — see https://pkg.go.dev/k8s.io/client-go/rest#Config
// ("Timeout: the maximum length of time to wait before giving up on a
// server request. A value of zero means no timeout."). A Timeout the
// kubeconfig sets itself wins.
const RequestTimeout = 30 * time.Second

// HasContext reports whether kubeconfig defines the context name. An
// error means the kubeconfig itself couldn't be read.
func HasContext(kubeconfig, name string) (bool, error) {
	raw, err := (&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}).Load()
	if err != nil {
		return false, fmt.Errorf("failed to load kubeconfig: %w", err)
	}
	_, ok := raw.Contexts[name]
	return ok, nil
}

// NewConfig loads a rest.Config from kubeconfig/context.
func NewConfig(kubeconfig, ctx string) (*rest.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: ctx}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, overrides,
	).ClientConfig()
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
	// Per-call deadline. Bounded so a broken cluster fails the
	// spinner in seconds, not 75+ seconds of TCP dial wait. Respect
	// an explicit Timeout already set on the config (loaded from
	// kubeconfig).
	if config.Timeout == 0 {
		config.Timeout = RequestTimeout
	}
	// Force exec auth plugins (oci-cli session token, aws-iam-authenticator,
	// etc.) into non-interactive mode. With Always/IfAvailable, client-go
	// pipes the plugin's stderr directly to os.Stderr when stdin is a tty —
	// which the TUI's alt-screen is — and any Python traceback or "session
	// expired" message corrupts the bubbletea frame, manifesting as dropped
	// keystrokes. Never makes client-go capture the plugin's stderr into a
	// buffer and surface it inside the returned error instead.
	if config.ExecProvider != nil {
		config.ExecProvider.InteractiveMode = clientcmdapi.NeverExecInteractiveMode
	}
	// Identify this client in user agent.
	rest.AddUserAgent(config, "toolkit")
	return config, nil
}

/*
NewWatchConfig is NewConfig for long-lived watch streams: identical except
Timeout is zero. client-go turns Config.Timeout into http.Client.Timeout,
which bounds reading the response body too — so a 30s Timeout cuts every
watch stream at 30s, before the server's ~1m bookmarks can advance the
resume resourceVersion; once etcd compacts past it, the reconnect fails
with 410 Expired and the watch dies. Watches are bounded instead by ctx
cancellation and the server-side TimeoutSeconds durableOpener sets.
*/
func NewWatchConfig(kubeconfig, ctx string) (*rest.Config, error) {
	config, err := NewConfig(kubeconfig, ctx)
	if err != nil {
		return nil, err
	}
	config.Timeout = 0
	return config, nil
}

/*
NewWatchClientsetFromKubeConfig is NewClientsetFromKubeConfig built on
NewWatchConfig, for clientsets that open watches.
*/
func NewWatchClientsetFromKubeConfig(kubeconfig, ctx string) (kubernetes.Interface, error) {
	config, err := NewWatchConfig(kubeconfig, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create config from kubeconfig: %w", err)
	}
	return NewClientsetFromRestConfig(config)
}

/*
NewWatchDynamicClientFromKubeConfig is NewDynamicClientFromKubeConfig built
on NewWatchConfig, for dynamic clients that open watches.
*/
func NewWatchDynamicClientFromKubeConfig(kubeconfig, ctx string) (dynamic.Interface, error) {
	config, err := NewWatchConfig(kubeconfig, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create config from kubeconfig: %w", err)
	}
	return NewDynamicClient(config)
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

/*
ServerVersion returns the API server's git version (e.g. "v1.30.1") for the
given kubeconfig context. Bounded by RequestTimeout via NewConfig.
*/
func ServerVersion(kubeconfig, ctx string) (string, error) {
	clientset, err := NewClientsetFromKubeConfig(kubeconfig, ctx)
	if err != nil {
		return "", err
	}
	info, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get server version: %w", err)
	}
	return info.GitVersion, nil
}
