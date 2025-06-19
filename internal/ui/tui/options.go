package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	logging "github.com/jingle2008/toolkit/internal/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// ModelOption defines a functional option for configuring Model.
type ModelOption func(*Model)

/*
WithContext sets the context.Context for the Model.
*/
func WithContext(ctx context.Context) ModelOption {
	return func(m *Model) {
		m.ctx = ctx
	}
}

// WithRepoPath sets the repoPath field.
func WithRepoPath(repoPath string) ModelOption {
	return func(m *Model) {
		m.repoPath = repoPath
	}
}

// WithKubeConfig sets the kubeConfig field.
func WithKubeConfig(kubeConfig string) ModelOption {
	return func(m *Model) {
		m.kubeConfig = kubeConfig
	}
}

// WithEnvironment sets the environment field.
func WithEnvironment(env models.Environment) ModelOption {
	return func(m *Model) {
		m.environment = env
	}
}

/*
WithCategory sets the category field for the Model.
*/
func WithCategory(category domain.Category) ModelOption {
	return func(m *Model) {
		m.category = category
	}
}

// WithTable sets the table.Model.
func WithTable(tbl *table.Model) ModelOption {
	return func(m *Model) {
		m.table = tbl
	}
}

// WithLoader sets the Loader implementation for the Model.
// The Loader interface must implement all loader interfaces (DatasetLoader, BaseModelLoader, GpuPoolLoader, GpuNodeLoader, DedicatedAIClusterLoader).
func WithLoader(l loader.Loader) ModelOption {
	return func(m *Model) {
		m.loader = l
	}
}

// WithLogger sets the logger for the Model.
func WithLogger(logger logging.Logger) ModelOption {
	return func(m *Model) {
		m.logger = logger
	}
}

// WithFilter sets a starting filter before Init().
func WithFilter(filter string) ModelOption {
	return func(m *Model) {
		m.newFilter = filter
	}
}

// WithVersion sets the version of the Model.
func WithVersion(v string) ModelOption {
	return func(m *Model) { m.version = v }
}
