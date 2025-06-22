package errors

import (
	stdErrors "errors"
	"fmt"
	"testing"
)

func TestWrapNil(t *testing.T) {
	if got := Wrap("action", nil); got != nil {
		t.Errorf("Wrap(nil) = %v, want nil", got)
	}
}

func TestWrapNonNil(t *testing.T) {
	base := fmt.Errorf("base error")
	wrapped := Wrap("action", base)
	if wrapped == nil {
		t.Fatal("Wrap(non-nil) = nil, want error")
	}
	if !stdErrors.Is(wrapped, base) {
		t.Errorf("errors.Is(wrapped, base) = false, want true")
	}
}

func TestJoin(t *testing.T) {
	e1 := fmt.Errorf("err1")
	e2 := fmt.Errorf("err2")
	joined := Join(e1, e2)
	if !stdErrors.Is(joined, e1) || !stdErrors.Is(joined, e2) {
		t.Errorf("errors.Is(joined, e1/e2) = false, want true")
	}
}

func TestUnwrap(t *testing.T) {
	base := fmt.Errorf("base")
	wrapped := Wrap("action", base)
	got := Unwrap(wrapped)
	if got == nil {
		t.Errorf("Unwrap(wrapped) = nil, want error")
	}
}

func TestErrUnknownSentinel(t *testing.T) {
	err := Wrap("foo", ErrUnknown)
	if !stdErrors.Is(err, ErrUnknown) {
		t.Errorf("errors.Is(err, ErrUnknown) = false, want true")
	}
	if stdErrors.Is(err, fmt.Errorf("other")) {
		t.Errorf("errors.Is(err, other) = true, want false")
	}
}
