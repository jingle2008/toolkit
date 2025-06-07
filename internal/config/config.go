package config

import (
	"flag"
	"os"
	"path/filepath"

	"errors"
	"fmt"

	domain "github.com/jingle2008/toolkit/internal/domain"
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

/*
Validate checks that all required fields in the Config are set and valid.
*/
func (c Config) Validate() error {
	if c.RepoPath == "" {
		return errors.New("config: RepoPath is required")
	}
	if c.KubeConfig == "" {
		return errors.New("config: KubeConfig is required")
	}
	if c.EnvType == "" {
		return errors.New("config: EnvType is required")
	}
	if c.EnvRegion == "" {
		return errors.New("config: EnvRegion is required")
	}
	if c.EnvRealm == "" {
		return errors.New("config: EnvRealm is required")
	}
	if c.Category == "" {
		return errors.New("config: Category is required")
	}
	_, err := domain.ParseCategory(c.Category)
	if err != nil {
		return fmt.Errorf("config: invalid category: %w", err)
	}
	return nil
}

/*
env returns the value of the environment variable for key, or def if not set.
*/
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Parse parses CLI flags and environment variables into a Config struct.
// Accepts args for testability; if args is nil, uses os.Args[1:].
func ParseArgs(args []string) Config {
	home := homedir.HomeDir()
	defaultKube := filepath.Join(home, ".kube", "config")

	fs := flag.NewFlagSet("toolkit", flag.ContinueOnError)
	repoPath := fs.String("repo", env("TOOLKIT_REPO_PATH", "/Users/jinguzha/Work/repos/genai-shepherd-flocks"), "Path to repo")
	kubeConfig := fs.String("kubeconfig", env("KUBECONFIG", defaultKube), "Path to kubeconfig")
	envType := fs.String("envtype", env("TOOLKIT_ENV_TYPE", "preprod"), "Environment type")
	envRegion := fs.String("envregion", env("TOOLKIT_ENV_REGION", "us-chicago-1"), "Environment region")
	envRealm := fs.String("envrealm", env("TOOLKIT_ENV_REALM", "oc1"), "Environment realm")
	category := fs.String("category", env("TOOLKIT_CATEGORY", "Tenant"), "Toolkit category")

	// If args is nil, use os.Args[1:]
	if args == nil {
		args = os.Args[1:]
	}
	_ = fs.Parse(args)

	return Config{
		RepoPath:   *repoPath,
		KubeConfig: *kubeConfig,
		EnvType:    *envType,
		EnvRegion:  *envRegion,
		EnvRealm:   *envRealm,
		Category:   *category,
	}
}

/*
Parse parses CLI flags and environment variables into a Config struct.
For backward compatibility.
*/
func Parse() Config {
	return ParseArgs(nil)
}
