package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestWrap(t *testing.T) {
	t.Parallel()
	base := errors.New("base")
	tests := []struct {
		name    string
		err     error
		msg     string
		wantNil bool
	}{
		{"nil error", nil, "context", true},
		{"non-nil error", base, "context", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			wrapped := Wrap(tt.err, tt.msg)
			if tt.wantNil && wrapped != nil {
				t.Errorf("expected nil, got %v", wrapped)
			}
			if !tt.wantNil {
				if wrapped == nil {
					t.Errorf("expected error, got nil")
				} else if !errors.Is(wrapped, base) {
					t.Errorf("wrapped error does not contain base error")
				} else if got, want := wrapped.Error(), fmt.Sprintf("%s: %s", tt.msg, base.Error()); got != want {
					t.Errorf("unexpected error string: got %q, want %q", got, want)
				}
			}
		})
	}
}

func TestJoin(t *testing.T) { //nolint: cyclop
	t.Parallel()
	err1 := errors.New("err1")
	err2 := errors.New("err2")
	tests := []struct {
		name string
		in   []error
		want error
	}{
		{"all nil", []error{nil, nil}, nil},
		{"one error", []error{err1, nil}, err1},
		{"two errors", []error{err1, err2}, nil}, // will check with errors.Is
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			got := Join(tt.in...)
			switch {
			case tt.want == nil && got != nil && len(tt.in) == 2 && tt.in[0] != nil && tt.in[1] != nil:
				// For two errors, errors.Join returns a joined error, not nil
				if !errors.Is(got, err1) || !errors.Is(got, err2) {
					t.Errorf("joined error does not contain both errors")
				}
			case tt.want == nil && got != nil && len(tt.in) == 2 && tt.in[0] == nil && tt.in[1] == nil:
				t.Errorf("expected nil, got %v", got)
			case tt.want != nil && !errors.Is(got, tt.want):
				t.Errorf("expected error %v, got %v", tt.want, got)
			}
		})
	}
}
