package common

import (
	"testing"
)

func TestEditTargetString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		val  EditTarget
		want string
	}{
		{NoneTarget, "none"},
		{FilterTarget, "filter"},
		{AliasTarget, "alias"},
		{EditTarget(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.val.String(); got != tt.want {
			t.Errorf("EditTarget(%d).String() = %q, want %q", tt.val, got, tt.want)
		}
	}
}

func TestInputModeString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		val  InputMode
		want string
	}{
		{UnknownInput, "unknown"},
		{EditInput, "edit"},
		{NormalInput, "normal"},
		{InputMode(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.val.String(); got != tt.want {
			t.Errorf("InputMode(%d).String() = %q, want %q", tt.val, got, tt.want)
		}
	}
}

func TestViewModeString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		val  ViewMode
		want string
	}{
		{ListView, "list"},
		{DetailsView, "details"},
		{LoadingView, "loading"},
		{HelpView, "help"},
		{ErrorView, "error"},
		{ViewMode(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.val.String(); got != tt.want {
			t.Errorf("ViewMode(%d).String() = %q, want %q", tt.val, got, tt.want)
		}
	}
}
