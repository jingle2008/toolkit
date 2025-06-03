package testutil

import (
	"testing"
)

func TestEqual(t *testing.T) {
	Equal(t, 1, 1)
}

func TestNotNil(t *testing.T) {
	NotNil(t, "not nil")
}

func TestContains(t *testing.T) {
	Contains(t, "hello world", "world")
}

func TestGreaterOrEqual(t *testing.T) {
	GreaterOrEqual(t, 5, 3)
}

func TestRequireEqual(t *testing.T) {
	RequireEqual(t, 1, 1)
}

func TestRequireNotNil(t *testing.T) {
	RequireNotNil(t, "not nil")
}

func TestRequireContains(t *testing.T) {
	RequireContains(t, "foo", "foo")
}

func TestRequireNoError(t *testing.T) {
	RequireNoError(t, nil)
}

func TestAssertPanic(t *testing.T) {
	AssertPanic(t, func() { panic("should panic") })
}
