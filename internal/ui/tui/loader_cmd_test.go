package tui

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/models"
)

type errLoader struct {
	fakeLoader
	err error
}

func (e errLoader) LoadLimitRegionalOverrides(_ context.Context, _ string, _ models.Environment) ([]models.LimitRegionalOverride, error) {
	return nil, e.err
}

func (e errLoader) LoadConsolePropertyRegionalOverrides(_ context.Context, _ string, _ models.Environment) ([]models.ConsolePropertyRegionalOverride, error) {
	return nil, e.err
}

func (e errLoader) LoadPropertyRegionalOverrides(_ context.Context, _ string, _ models.Environment) ([]models.PropertyRegionalOverride, error) {
	return nil, e.err
}

// regionalLoad runs cat's load on a model over ld.
func regionalLoad(t *testing.T, ld loader.Composite, cat domain.Category, gen int) tea.Msg {
	t.Helper()
	m := newTestModel(t)
	m.loader = ld
	return m.loadCategoryCmd(cat, gen)()
}

var regionalOverrideCategories = []domain.Category{
	domain.LimitRegionalOverride, domain.ConsolePropertyRegionalOverride, domain.PropertyRegionalOverride,
}

func TestLoadRegionalOverridesCmds_Success(t *testing.T) {
	t.Parallel()
	for _, cat := range regionalOverrideCategories {
		got, ok := regionalLoad(t, fakeLoader{}, cat, 7).(categoryLoadedMsg)
		require.Truef(t, ok, "%s: want its rows", cat)
		assert.Equal(t, cat, got.Cat)
		assert.Equal(t, 7, got.Gen)
	}
}

func TestLoadRegionalOverridesCmds_Error(t *testing.T) {
	t.Parallel()
	for _, cat := range regionalOverrideCategories {
		got, ok := regionalLoad(t, errLoader{err: errors.New("boom")}, cat, 1).(errMsg)
		require.Truef(t, ok, "%s: want an errMsg", cat)
		assert.Equal(t, cat, got.Cat)
	}
}
