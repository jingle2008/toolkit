package toolkit

import (
	"context"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/jingle2008/toolkit/internal/app/toolkit/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// ModelOption defines a functional option for configuring Model.
type ModelOption func(*Model)

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

func WithCategory(category domain.Category) ModelOption {
	return func(m *Model) {
		m.category = category
	}
}

// WithAppContext sets the context field.
func WithAppContext(ctx *domain.AppContext) ModelOption {
	return func(m *Model) {
		m.context = ctx
	}
}

// WithViewSize sets the viewWidth and viewHeight fields.
func WithViewSize(width, height int) ModelOption {
	return func(m *Model) {
		m.viewWidth = width
		m.viewHeight = height
	}
}

// WithHelp sets the help.Model.
func WithHelp(helpModel *help.Model) ModelOption {
	return func(m *Model) {
		m.help = helpModel
	}
}

// WithTable sets the table.Model.
func WithTable(tbl *table.Model) ModelOption {
	return func(m *Model) {
		m.table = tbl
	}
}

// WithTextInput sets the textinput.Model.
func WithTextInput(ti *textinput.Model) ModelOption {
	return func(m *Model) {
		m.textInput = ti
	}
}

// WithViewport sets the viewport.Model.
func WithViewport(vp *viewport.Model) ModelOption {
	return func(m *Model) {
		m.viewport = vp
	}
}

// WithRenderer sets the Renderer implementation for the Model.
func WithRenderer(r Renderer) ModelOption {
	return func(m *Model) {
		m.renderer = r
	}
}

// WithLoader sets the Loader implementation for the Model.
// The Loader interface must implement all loader interfaces (DatasetLoader, BaseModelLoader, GpuPoolLoader, GpuNodeLoader, DedicatedAIClusterLoader).
func WithLoader(loader Loader) ModelOption {
	return func(m *Model) {
		m.loader = loader
	}
}

/*
WithContext sets the context.Context for the Model.
*/
func WithContext(ctx context.Context) ModelOption {
	return func(m *Model) {
		m.contextCtx = ctx
	}
}
