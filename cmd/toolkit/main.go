/*
Command toolkit is the CLI entry-point for the toolkit application.
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/app/toolkit"
	"github.com/jingle2008/toolkit/internal/app/toolkit/domain"
	"github.com/jingle2008/toolkit/internal/app/toolkit/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"go.uber.org/zap"
	"k8s.io/client-go/util/homedir"
)

/*
Config holds configuration for the toolkit CLI application.
*/
type Config struct {
	RepoPath   string
	KubeConfig string
	EnvType    string
	EnvRegion  string
	EnvRealm   string
	Category   string
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseConfig() Config {
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

func categoryFromString(s string) (domain.Category, error) {
	switch strings.ToLower(s) {
	case "tenant":
		return domain.Tenant, nil
	case "limitdefinition":
		return domain.LimitDefinition, nil
	case "consolepropertydefinition":
		return domain.ConsolePropertyDefinition, nil
	case "propertydefinition":
		return domain.PropertyDefinition, nil
	case "limittenancyoverride":
		return domain.LimitTenancyOverride, nil
	case "consolepropertytenancyoverride":
		return domain.ConsolePropertyTenancyOverride, nil
	case "propertytenancyoverride":
		return domain.PropertyTenancyOverride, nil
	case "consolepropertyregionaloverride":
		return domain.ConsolePropertyRegionalOverride, nil
	case "propertyregionaloverride":
		return domain.PropertyRegionalOverride, nil
	case "basemodel":
		return domain.BaseModel, nil
	case "modelartifact":
		return domain.ModelArtifact, nil
	case "environment":
		return domain.Environment, nil
	case "servicetenancy":
		return domain.ServiceTenancy, nil
	case "gpupool":
		return domain.GpuPool, nil
	case "gpunode":
		return domain.GpuNode, nil
	case "dedicatedaicluster":
		return domain.DedicatedAICluster, nil
	default:
		return 0, fmt.Errorf("invalid category: %q", s)
	}
}

func run(ctx context.Context, cfg Config) error {
	category, err := categoryFromString(cfg.Category)
	if err != nil {
		valid := []string{
			"Tenant", "LimitDefinition", "ConsolePropertyDefinition", "PropertyDefinition",
			"LimitTenancyOverride", "ConsolePropertyTenancyOverride", "PropertyTenancyOverride",
			"ConsolePropertyRegionalOverride", "PropertyRegionalOverride", "BaseModel", "ModelArtifact",
			"Environment", "ServiceTenancy", "GpuPool", "GpuNode", "DedicatedAICluster",
		}
		return fmt.Errorf("invalid category %q. Valid categories are: %s", cfg.Category, strings.Join(valid, ", "))
	}
	env := models.Environment{
		Type:   cfg.EnvType,
		Region: cfg.EnvRegion,
		Realm:  cfg.EnvRealm,
	}
	repoPath := cfg.RepoPath
	kubeConfig := cfg.KubeConfig

	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		return fmt.Errorf("fatal: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Printf("WARN: closing debug log: %v\n", err)
		}
	}()

	logger, _ := zap.NewProduction()
	ctx = logging.CtxWithLogger(ctx, logger)
	model := toolkit.NewModel(
		toolkit.WithRepoPath(repoPath),
		toolkit.WithKubeConfig(kubeConfig),
		toolkit.WithEnvironment(env),
		toolkit.WithCategory(category),
		toolkit.WithContext(ctx),
	)
	p := tea.NewProgram(model, tea.WithAltScreen())
	// Run the program with context cancellation
	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("alas, there's been an error: %v", err)
		}
	}
	return nil
}

func main() {
	cfg := parseConfig()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg); err != nil {
		log.Printf("%v", err)
		stop()
	}
}
