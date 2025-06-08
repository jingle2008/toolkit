/*
Package config provides configuration loading and validation for the toolkit CLI application.
*/
package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	domain "github.com/jingle2008/toolkit/internal/domain"
	"k8s.io/client-go/util/homedir"
)

// Config holds configuration for the toolkit CLI application.
type Config struct {
	ConfigFile string `json:"config_file" yaml:"config_file"`
	RepoPath   string `json:"repo_path" yaml:"repo_path"`
	KubeConfig string `json:"kubeconfig" yaml:"kubeconfig"`
	EnvType    string `json:"env_type" yaml:"env_type"`
	EnvRegion  string `json:"env_region" yaml:"env_region"`
	EnvRealm   string `json:"env_realm" yaml:"env_realm"`
	Category   string `json:"category" yaml:"category"`
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

// loadConfigFile loads config from a YAML or JSON file.
func loadConfigFile(path string) (Config, error) {
	f, err := os.Open(path) // #nosec G304
	if err != nil {
		return Config{}, err
	}
	defer func() {
		_ = f.Close()
	}()
	var cfg Config
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		dec := yaml.NewDecoder(f)
		if err := dec.Decode(&cfg); err != nil {
			return Config{}, err
		}
	case ".json":
		dec := json.NewDecoder(f)
		if err := dec.Decode(&cfg); err != nil {
			return Config{}, err
		}
	default:
		return Config{}, fmt.Errorf("unsupported config file extension: %s", ext)
	}
	return cfg, nil
}

// mergeConfig overlays non-empty fields from src onto dst.
func mergeConfig(dst, src Config) Config {
	if src.ConfigFile != "" {
		dst.ConfigFile = src.ConfigFile
	}
	if src.RepoPath != "" {
		dst.RepoPath = src.RepoPath
	}
	if src.KubeConfig != "" {
		dst.KubeConfig = src.KubeConfig
	}
	if src.EnvType != "" {
		dst.EnvType = src.EnvType
	}
	if src.EnvRegion != "" {
		dst.EnvRegion = src.EnvRegion
	}
	if src.EnvRealm != "" {
		dst.EnvRealm = src.EnvRealm
	}
	if src.Category != "" {
		dst.Category = src.Category
	}
	return dst
}

// Parse parses CLI flags, environment variables, and optionally a config file into a Config struct.
// Accepts args for testability.
func Parse(args []string) Config {
	home := homedir.HomeDir()
	defaultKube := filepath.Join(home, ".kube", "config")

	fs := flag.NewFlagSet("toolkit", flag.ContinueOnError)
	configFile := fs.String("config", env("TOOLKIT_CONFIG", ""), "Path to config file (YAML or JSON)")
	repoPath := fs.String("repo", env("TOOLKIT_REPO_PATH", "/Users/jinguzha/Work/repos/genai-shepherd-flocks"), "Path to repo")
	kubeConfig := fs.String("kubeconfig", env("KUBECONFIG", defaultKube), "Path to kubeconfig")
	envType := fs.String("envtype", env("TOOLKIT_ENV_TYPE", "preprod"), "Environment type")
	envRegion := fs.String("envregion", env("TOOLKIT_ENV_REGION", "us-chicago-1"), "Environment region")
	envRealm := fs.String("envrealm", env("TOOLKIT_ENV_REALM", "oc1"), "Environment realm")
	category := fs.String("category", env("TOOLKIT_CATEGORY", "Tenant"), "Toolkit category")

	_ = fs.Parse(args)

	// Start with config from file if provided
	var cfg Config
	if *configFile != "" {
		fileCfg, err := loadConfigFile(*configFile)
		if err == nil {
			cfg = fileCfg
		}
		cfg.ConfigFile = *configFile
	}

	// Overlay with flags/envs (flags take precedence)
	flagCfg := Config{
		ConfigFile: *configFile,
		RepoPath:   *repoPath,
		KubeConfig: *kubeConfig,
		EnvType:    *envType,
		EnvRegion:  *envRegion,
		EnvRealm:   *envRealm,
		Category:   *category,
	}
	cfg = mergeConfig(cfg, flagCfg)

	return cfg
}
