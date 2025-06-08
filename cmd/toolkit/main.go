/*
Command toolkit is the CLI entry-point for the toolkit application.
*/
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	logging "github.com/jingle2008/toolkit/internal/infra/logging"
	tui "github.com/jingle2008/toolkit/internal/ui/tui"
	"github.com/jingle2008/toolkit/pkg/models"
)

func categoryFromString(s string) (domain.Category, error) {
	return domain.ParseCategory(s)
}

func run(ctx context.Context, logger logging.Logger, cfg config.Config) error {
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

	ctx = logging.WithContext(ctx, logger)
	logger.Infow("starting toolkit",
		"repo", repoPath,
		"env", env,
		"category", category,
	)

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
		logger.Errorw("failed to create toolkit model", "error", err)
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
		logger.Errorw("context cancelled", "error", ctx.Err())
		return ctx.Err()
	case err := <-done:
		if err != nil {
			logger.Errorw("program error", "error", err)
			return fmt.Errorf("alas, there's been an error: %v", err)
		}
	}
	return nil
}

func main() {
	cfg := config.Parse()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(2)
	}
	logger, err := logging.NewLogger(false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx = logging.WithContext(ctx, logger)
	if err := run(ctx, logger, cfg); err != nil {
		logger.Errorw("fatal error", "error", err)
		os.Exit(1)
	}
}
