/*
Package testutil provides assertion helpers for testing, wrapping testify's assert and require.
*/
package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/* Assert wrappers */

// Equal asserts that expected and actual are equal.
func Equal(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	return assert.Equal(t, expected, actual, msgAndArgs...)
}

// NotNil asserts that object is not nil.
func NotNil(t *testing.T, object interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	return assert.NotNil(t, object, msgAndArgs...)
}

// Contains asserts that s contains contains.
func Contains(t *testing.T, s, contains interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	return assert.Contains(t, s, contains, msgAndArgs...)
}

// GreaterOrEqual asserts that e1 is greater than or equal to e2.
func GreaterOrEqual(t *testing.T, e1, e2 interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	return assert.GreaterOrEqual(t, e1, e2, msgAndArgs...)
}

/* Require wrappers */

// RequireEqual requires that expected and actual are equal.
func RequireEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	require.Equal(t, expected, actual, msgAndArgs...)
}

// RequireNotNil requires that object is not nil.
func RequireNotNil(t *testing.T, object interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	require.NotNil(t, object, msgAndArgs...)
}

// RequireContains requires that s contains contains.
func RequireContains(t *testing.T, s, contains interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	require.Contains(t, s, contains, msgAndArgs...)
}

// RequireNoError requires that err is nil.
func RequireNoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	require.NoError(t, err, msgAndArgs...)
}

/* AssertPanic helper */

// AssertPanic asserts that fn panics.
func AssertPanic(t *testing.T, fn func(), msgAndArgs ...interface{}) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			require.Fail(t, "expected panic but function did not panic", msgAndArgs...)
		}
	}()
	fn()
}
