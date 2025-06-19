/*
Package testutil provides assertion helpers for testing, wrapping testify's assert and require.
*/
package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
Equal asserts that expected and actual are equal.

Parameters:
  - t: the testing.T instance
  - expected: the expected value
  - actual: the actual value
  - msgAndArgs: optional message and arguments

Returns:
  - bool: true if equal, false otherwise
*/
func Equal(t *testing.T, expected, actual any, msgAndArgs ...any) bool {
	t.Helper()
	return assert.Equal(t, expected, actual, msgAndArgs...)
}

// NotNil asserts that object is not nil.
func NotNil(t *testing.T, object any, msgAndArgs ...any) bool {
	t.Helper()
	return assert.NotNil(t, object, msgAndArgs...)
}

// Contains asserts that s contains contains.
func Contains(t *testing.T, s, contains any, msgAndArgs ...any) bool {
	t.Helper()
	return assert.Contains(t, s, contains, msgAndArgs...)
}

/*
ElementsMatch asserts that two slices contain the same elements, regardless of order.
*/
func ElementsMatch(t *testing.T, listA, listB any, msgAndArgs ...any) bool {
	t.Helper()
	return assert.ElementsMatch(t, listA, listB, msgAndArgs...)
}

// GreaterOrEqual asserts that e1 is greater than or equal to e2.
func GreaterOrEqual(t *testing.T, e1, e2 any, msgAndArgs ...any) bool {
	t.Helper()
	return assert.GreaterOrEqual(t, e1, e2, msgAndArgs...)
}

/* Require wrappers */

// RequireEqual requires that expected and actual are equal.
func RequireEqual(t *testing.T, expected, actual any, msgAndArgs ...any) {
	t.Helper()
	require.Equal(t, expected, actual, msgAndArgs...)
}

// RequireNotNil requires that object is not nil.
func RequireNotNil(t *testing.T, object any, msgAndArgs ...any) {
	t.Helper()
	require.NotNil(t, object, msgAndArgs...)
}

/*
RequireElementsMatch requires that two slices contain the same elements, regardless of order.
*/
func RequireElementsMatch(t *testing.T, listA, listB any, msgAndArgs ...any) {
	t.Helper()
	require.ElementsMatch(t, listA, listB, msgAndArgs...)
}

// RequireContains requires that s contains contains.
func RequireContains(t *testing.T, s, contains any, msgAndArgs ...any) {
	t.Helper()
	require.Contains(t, s, contains, msgAndArgs...)
}

/*
RequireError requires that err is not nil.
*/
func RequireError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	require.Error(t, err, msgAndArgs...)
}

// RequireNoError requires that err is nil.
func RequireNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	require.NoError(t, err, msgAndArgs...)
}

/* AssertPanic helper */

// AssertPanic asserts that fn panics.
func AssertPanic(t *testing.T, fn func(), msgAndArgs ...any) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			require.Fail(t, "expected panic but function did not panic", msgAndArgs...)
		}
	}()
	fn()
}
