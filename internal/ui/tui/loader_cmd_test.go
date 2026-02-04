package tui

import (
	"context"
	"errors"
	"testing"

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

func TestLoadRegionalOverridesCmds_Success(t *testing.T) {
	t.Parallel()
	ld := fakeLoader{}
	ctx := context.Background()
	env := models.Environment{}
	gen := 7

	msg := loadLimitRegionalOverridesCmd(ctx, ld, "repo", env, gen)()
	if got, ok := msg.(limitRegionalOverridesLoadedMsg); !ok || got.Gen != gen {
		t.Fatalf("unexpected message: %#v", msg)
	}

	msg = loadConsolePropertyRegionalOverridesCmd(ctx, ld, "repo", env, gen)()
	if got, ok := msg.(consolePropertyRegionalOverridesLoadedMsg); !ok || got.Gen != gen {
		t.Fatalf("unexpected message: %#v", msg)
	}

	msg = loadPropertyRegionalOverridesCmd(ctx, ld, "repo", env, gen)()
	if got, ok := msg.(propertyRegionalOverridesLoadedMsg); !ok || got.Gen != gen {
		t.Fatalf("unexpected message: %#v", msg)
	}
}

func TestLoadRegionalOverridesCmds_Error(t *testing.T) {
	t.Parallel()
	ld := errLoader{err: errors.New("boom")}
	ctx := context.Background()
	env := models.Environment{}

	if msg := loadLimitRegionalOverridesCmd(ctx, ld, "repo", env, 1)(); msg == nil {
		t.Fatal("expected ErrMsg, got nil")
	}
	if _, ok := loadConsolePropertyRegionalOverridesCmd(ctx, ld, "repo", env, 1)().(ErrMsg); !ok {
		t.Fatal("expected ErrMsg from console property override loader")
	}
	if _, ok := loadPropertyRegionalOverridesCmd(ctx, ld, "repo", env, 1)().(ErrMsg); !ok {
		t.Fatal("expected ErrMsg from property override loader")
	}
}
