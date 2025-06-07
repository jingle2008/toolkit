/*
Command toolkit is the CLI entry-point for the toolkit application.
*/
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	logctx "github.com/jingle2008/toolkit/internal/infra/logging"
	tui "github.com/jingle2008/toolkit/internal/ui/tui"
	"github.com/jingle2008/toolkit/pkg/models"
	"go.uber.org/zap"
)

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

func run(ctx context.Context, cfg config.Config) error {
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
	ctx = logctx.CtxWithLogger(ctx, logger)
	model, err := tui.NewModel(
		tui.WithRepoPath(repoPath),
		tui.WithKubeConfig(kubeConfig),
		tui.WithEnvironment(env),
		tui.WithCategory(category),
		tui.WithContext(ctx),
		tui.WithLogger(logger),
		tui.WithLoader(loader.ProductionLoader{}),
	)
	if err != nil {
		return fmt.Errorf("failed to create toolkit model: %w", err)
	}
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
	cfg := config.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg); err != nil {
		log.Fatal(err)
	}
}
