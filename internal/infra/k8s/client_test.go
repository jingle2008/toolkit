package k8s

import (
	"path/filepath"
	"testing"

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

func TestHasContext(t *testing.T) {
	t.Parallel()
	cfg := api.NewConfig()
	cfg.Clusters["c"] = &api.Cluster{Server: "https://127.0.0.1"}
	cfg.Contexts["dp-ppe-ord"] = &api.Context{Cluster: "c"}
	path := writeTempKubeconfig(t, cfg)

	ok, err := HasContext(path, "dp-ppe-ord")
	if err != nil || !ok {
		t.Fatalf("HasContext(dp-ppe-ord) = %v, %v; want true", ok, err)
	}
	ok, err = HasContext(path, "dp-dev-phx")
	if err != nil || ok {
		t.Fatalf("HasContext(dp-dev-phx) = %v, %v; want false", ok, err)
	}
	if _, err := HasContext("/nonexistent/kubeconfig", "x"); err == nil {
		t.Fatal("HasContext on a missing kubeconfig: want an error")
	}
}

func TestNewConfig_AppliesRequestTimeout(t *testing.T) { //nolint:paralleltest // paralleltest is not supported in this package
	validConfig := api.NewConfig()
	validConfig.Clusters["test"] = &api.Cluster{Server: "https://127.0.0.1"}
	validConfig.Contexts["test"] = &api.Context{Cluster: "test", AuthInfo: "user"}
	validConfig.AuthInfos["user"] = &api.AuthInfo{}
	validConfig.CurrentContext = "test"
	path := writeTempKubeconfig(t, validConfig)

	cfg, err := NewConfig(path, "test")
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	if cfg.Timeout != RequestTimeout {
		t.Errorf("default Timeout = %v, want %v", cfg.Timeout, RequestTimeout)
	}
}

// TestNewWatchConfig_NoTimeout guards the watch/request split: client-go
// applies Config.Timeout to the whole HTTP exchange, body included, so a
// watch config must have none while NewConfig still bounds regular calls.
func TestNewWatchConfig_NoTimeout(t *testing.T) { //nolint:paralleltest // paralleltest is not supported in this package
	validConfig := api.NewConfig()
	validConfig.Clusters["test"] = &api.Cluster{Server: "https://127.0.0.1"}
	validConfig.Contexts["test"] = &api.Context{Cluster: "test", AuthInfo: "user"}
	validConfig.AuthInfos["user"] = &api.AuthInfo{}
	validConfig.CurrentContext = "test"
	path := writeTempKubeconfig(t, validConfig)

	cfg, err := NewConfig(path, "test")
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	if cfg.Timeout != RequestTimeout {
		t.Errorf("NewConfig Timeout = %v, want %v", cfg.Timeout, RequestTimeout)
	}
	wcfg, err := NewWatchConfig(path, "test")
	if err != nil {
		t.Fatalf("NewWatchConfig: %v", err)
	}
	if wcfg.Timeout != 0 {
		t.Errorf("NewWatchConfig Timeout = %v, want 0", wcfg.Timeout)
	}
	if wcfg.QPS != cfg.QPS || wcfg.Burst != cfg.Burst || wcfg.UserAgent != cfg.UserAgent {
		t.Error("NewWatchConfig must otherwise match NewConfig")
	}

	if _, err := NewWatchConfig("/nonexistent/path", "test"); err == nil {
		t.Error("expected error for bad kubeconfig path")
	}
	if _, err := NewWatchClientsetFromKubeConfig(path, "test"); err != nil {
		t.Errorf("NewWatchClientsetFromKubeConfig: %v", err)
	}
	if _, err := NewWatchClientsetFromKubeConfig("/nonexistent/path", "test"); err == nil {
		t.Error("expected error for bad kubeconfig path")
	}
	if _, err := NewWatchDynamicClientFromKubeConfig(path, "test"); err != nil {
		t.Errorf("NewWatchDynamicClientFromKubeConfig: %v", err)
	}
	if _, err := NewWatchDynamicClientFromKubeConfig("/nonexistent/path", "test"); err == nil {
		t.Error("expected error for bad kubeconfig path")
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
