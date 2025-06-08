package testutil

import (
	"errors"
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

func TestElementsMatch(t *testing.T) {
	t.Parallel()
	ElementsMatch(t, []int{1, 2, 3}, []int{3, 2, 1})
}

func TestRequireElementsMatch(t *testing.T) {
	t.Parallel()
	RequireElementsMatch(t, []string{"a", "b"}, []string{"b", "a"})
}

func TestRequireError(t *testing.T) {
	t.Parallel()
	RequireError(t, errors.New("fail"))
}
