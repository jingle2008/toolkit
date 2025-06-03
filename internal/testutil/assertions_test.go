package testutil

import (
	"testing"
)

func TestEqual(t *testing.T) {
	t.Parallel()
	Equal(t, 1, 1)
}

func TestNotNil(t *testing.T) {
	t.Parallel()
	NotNil(t, "not nil")
}

func TestContains(t *testing.T) {
	t.Parallel()
	Contains(t, "hello world", "world")
}

func TestGreaterOrEqual(t *testing.T) {
	t.Parallel()
	GreaterOrEqual(t, 5, 3)
}

func TestRequireEqual(t *testing.T) {
	t.Parallel()
	RequireEqual(t, 1, 1)
}

func TestRequireNotNil(t *testing.T) {
	t.Parallel()
	RequireNotNil(t, "not nil")
}

func TestRequireContains(t *testing.T) {
	t.Parallel()
	RequireContains(t, "foo", "foo")
}

func TestRequireNoError(t *testing.T) {
	t.Parallel()
	RequireNoError(t, nil)
}

func TestAssertPanic(t *testing.T) {
	t.Parallel()
	AssertPanic(t, func() { panic("should panic") })
}
