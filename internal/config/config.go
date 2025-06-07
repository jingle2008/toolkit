package config

import (
	"flag"
	"os"
	"path/filepath"

	"k8s.io/client-go/util/homedir"
)

// Config holds configuration for the toolkit CLI application.
type Config struct {
	RepoPath   string
	KubeConfig string
	EnvType    string
	EnvRegion  string
	EnvRealm   string
	Category   string
}

// getEnvOrDefault returns the value of the environment variable or the default.
func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Parse parses CLI flags and environment variables into a Config struct.
func Parse() Config {
	home := homedir.HomeDir()
	defaultKube := filepath.Join(home, ".kube", "config")

	repoPath := flag.String("repo", getEnvOrDefault("TOOLKIT_REPO_PATH", "/Users/jinguzha/Work/repos/genai-shepherd-flocks"), "Path to repo")
	kubeConfig := flag.String("kubeconfig", getEnvOrDefault("KUBECONFIG", defaultKube), "Path to kubeconfig")
	envType := flag.String("envtype", getEnvOrDefault("TOOLKIT_ENV_TYPE", "preprod"), "Environment type")
	envRegion := flag.String("envregion", getEnvOrDefault("TOOLKIT_ENV_REGION", "us-chicago-1"), "Environment region")
	envRealm := flag.String("envrealm", getEnvOrDefault("TOOLKIT_ENV_REALM", "oc1"), "Environment realm")
	category := flag.String("category", getEnvOrDefault("TOOLKIT_CATEGORY", "Tenant"), "Toolkit category")

	flag.Parse()

	return Config{
		RepoPath:   *repoPath,
		KubeConfig: *kubeConfig,
		EnvType:    *envType,
		EnvRegion:  *envRegion,
		EnvRealm:   *envRealm,
		Category:   *category,
	}
}
