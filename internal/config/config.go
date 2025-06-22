/*
Package config provides configuration loading and validation for the toolkit CLI application.
*/
package config

import (
	"errors"

	domain "github.com/jingle2008/toolkit/internal/domain"
	interrors "github.com/jingle2008/toolkit/internal/errors"
)

// Config holds configuration for the toolkit CLI application.
type Config struct {
	RepoPath   string `mapstructure:"repo_path"`
	KubeConfig string `mapstructure:"kubeconfig"`
	EnvType    string `mapstructure:"env_type"`
	EnvRegion  string `mapstructure:"env_region"`
	EnvRealm   string `mapstructure:"env_realm"`
	Category   string `mapstructure:"category"`
	LogFile    string `mapstructure:"log_file"`
	Debug      bool   `mapstructure:"debug"`
	Filter     string `mapstructure:"filter"`
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
		return interrors.Wrap("config: invalid category", err)
	}
	return nil
}
