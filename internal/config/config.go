/*
Package config provides configuration loading and validation for the toolkit CLI application.
*/
package config

import (
	"errors"
	"fmt"

	domain "github.com/jingle2008/toolkit/internal/domain"
)

// Config holds configuration for the toolkit CLI application.
type Config struct {
	RepoPath     string `mapstructure:"repo-path"`
	KubeConfig   string `mapstructure:"kubeconfig"`
	EnvType      string `mapstructure:"env-type"`
	EnvRegion    string `mapstructure:"env-region"`
	EnvRealm     string `mapstructure:"env-realm"`
	Category     string `mapstructure:"category"`
	LogFile      string `mapstructure:"log-file"`
	Debug        bool   `mapstructure:"debug"`
	Filter       string `mapstructure:"filter"`
	MetadataFile string `mapstructure:"metadata-file"`
	// MutationEnvOverrideAllowed opts MCP mutation tools into per-call
	// env_type / env_region / env_realm overrides. Default false: tools
	// silently fall back to the startup env, even if the agent provides
	// override fields. The flip is meaningful: with the override
	// allowed, a prompt-injection attack that sets confirm=true also
	// controls which realm is targeted, so the operator's kubeconfig /
	// OCI credentials decide the maximum blast radius — not the
	// operator's startup-env choice.
	MutationEnvOverrideAllowed bool `mapstructure:"mutation-env-override-allowed"`
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
