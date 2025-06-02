package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/* Assert wrappers */

func Equal(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	return assert.Equal(t, expected, actual, msgAndArgs...)
}

func NotNil(t *testing.T, object interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	return assert.NotNil(t, object, msgAndArgs...)
}

func Contains(t *testing.T, s, contains interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	return assert.Contains(t, s, contains, msgAndArgs...)
}

func GreaterOrEqual(t *testing.T, e1, e2 interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	return assert.GreaterOrEqual(t, e1, e2, msgAndArgs...)
}

/* Require wrappers */

func RequireEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	require.Equal(t, expected, actual, msgAndArgs...)
}

func RequireNotNil(t *testing.T, object interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	require.NotNil(t, object, msgAndArgs...)
}

func RequireContains(t *testing.T, s, contains interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	require.Contains(t, s, contains, msgAndArgs...)
}

func RequireNoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	require.NoError(t, err, msgAndArgs...)
}

/* AssertPanic helper */

func AssertPanic(t *testing.T, fn func(), msgAndArgs ...interface{}) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			require.Fail(t, "expected panic but function did not panic", msgAndArgs...)
		}
	}()
	fn()
}
