package k8s

import (
	"path/filepath"
	"testing"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func writeTempKubeconfig(t *testing.T, config *api.Config) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")
	err := clientcmd.WriteToFile(*config, path)
	if err != nil {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}
	return path
}

func TestNewConfig(t *testing.T) { //nolint:paralleltest // paralleltest is not supported in this package
	validConfig := api.NewConfig()
	validConfig.Clusters["test"] = &api.Cluster{Server: "https://127.0.0.1"}
	validConfig.Contexts["test"] = &api.Context{Cluster: "test", AuthInfo: "user"}
	validConfig.AuthInfos["user"] = &api.AuthInfo{}
	validConfig.CurrentContext = "test"

	kubeconfigPath := writeTempKubeconfig(t, validConfig)

	tests := []struct {
		name       string
		kubeconfig string
		ctx        string
		wantErr    bool
	}{
		{"valid config", kubeconfigPath, "test", false},
		{"bad path", "/nonexistent/path", "test", true},
		{"bad context", kubeconfigPath, "missing", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := NewConfig(tt.kubeconfig, tt.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && cfg == nil {
				t.Errorf("NewConfig() got nil config")
			}
		})
	}
}

func TestNewConfig_AppliesRequestTimeout(t *testing.T) { //nolint:paralleltest // mutates package-level RequestTimeout
	validConfig := api.NewConfig()
	validConfig.Clusters["test"] = &api.Cluster{Server: "https://127.0.0.1"}
	validConfig.Contexts["test"] = &api.Context{Cluster: "test", AuthInfo: "user"}
	validConfig.AuthInfos["user"] = &api.AuthInfo{}
	validConfig.CurrentContext = "test"
	path := writeTempKubeconfig(t, validConfig)

	// Default branch.
	cfg, err := NewConfig(path, "test")
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	if cfg.Timeout != DefaultRequestTimeout {
		t.Errorf("default Timeout = %v, want %v", cfg.Timeout, DefaultRequestTimeout)
	}

	// Override branch: setting RequestTimeout before NewConfig is the
	// supported way to tighten or relax the bound at startup.
	t.Cleanup(func() { RequestTimeout = DefaultRequestTimeout })
	RequestTimeout = 5 * time.Second
	cfg, err = NewConfig(path, "test")
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("override Timeout = %v, want %v", cfg.Timeout, 5*time.Second)
	}

	// Zero disables — matches client-go's "no timeout" semantics so
	// operators with explicit long-running calls can opt out.
	RequestTimeout = 0
	cfg, err = NewConfig(path, "test")
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	if cfg.Timeout != 0 {
		t.Errorf("zero override Timeout = %v, want 0", cfg.Timeout)
	}
}

func TestNewClientsetFromRestConfig(t *testing.T) {
	t.Parallel()
	// Use a minimal valid rest.Config for fake client
	cfg := &rest.Config{Host: "https://127.0.0.1"}
	clientset, err := NewClientsetFromRestConfig(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if clientset == nil {
		t.Error("expected non-nil clientset")
	}

	// Error path: pass nil config
	_, err = NewClientsetFromRestConfig(nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestNewClientsetFromKubeConfig(t *testing.T) { //nolint:paralleltest // paralleltest is not supported in this package
	validConfig := api.NewConfig()
	validConfig.Clusters["test"] = &api.Cluster{Server: "https://127.0.0.1"}
	validConfig.Contexts["test"] = &api.Context{Cluster: "test", AuthInfo: "user"}
	validConfig.AuthInfos["user"] = &api.AuthInfo{}
	validConfig.CurrentContext = "test"

	kubeconfigPath := writeTempKubeconfig(t, validConfig)

	_, err := NewClientsetFromKubeConfig(kubeconfigPath, "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = NewClientsetFromKubeConfig("/nonexistent/path", "test")
	if err == nil {
		t.Error("expected error for bad kubeconfig path")
	}
}

func TestNewDynamicClient(t *testing.T) {
	t.Parallel()
	// Use a minimal valid rest.Config for fake dynamic client
	cfg := &rest.Config{Host: "https://127.0.0.1"}
	_, err := NewDynamicClient(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Error path: pass nil config
	_, err = NewDynamicClient(nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestNewDynamicClientFromKubeConfig(t *testing.T) { //nolint:paralleltest // paralleltest is not supported in this package
	validConfig := api.NewConfig()
	validConfig.Clusters["test"] = &api.Cluster{Server: "https://127.0.0.1"}
	validConfig.Contexts["test"] = &api.Context{Cluster: "test", AuthInfo: "user"}
	validConfig.AuthInfos["user"] = &api.AuthInfo{}
	validConfig.CurrentContext = "test"

	kubeconfigPath := writeTempKubeconfig(t, validConfig)

	_, err := NewDynamicClientFromKubeConfig(kubeconfigPath, "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = NewDynamicClientFromKubeConfig("/nonexistent/path", "test")
	if err == nil {
		t.Error("expected error for bad kubeconfig path")
	}
}
