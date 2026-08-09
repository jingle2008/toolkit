package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegion_GetCode(t *testing.T) {
	t.Parallel()
	var r Region = "us-phoenix-1"
	assert.Equal(t, "phx", r.Code())
	r = "us-ashburn-1"
	assert.Equal(t, "iad", r.Code())

	// Mapped explicitly after SDK v65.114 added Newark.
	r = "us-newark-1"
	assert.Equal(t, "pgc", r.Code())

	// Sovereign-cloud / unmapped region: falls back to the city
	// segment instead of literal "UNKNOWN" so the table stays
	// identifiable until an explicit mapping is added.
	r = "ap-westtokyo-1"
	assert.Equal(t, "westtokyo", r.Code())

	// Input doesn't look like a region identifier — still UNKNOWN.
	r = "unknown-region"
	assert.Equal(t, "UNKNOWN", r.Code())
}

func TestCodeToRegion(t *testing.T) {
	t.Parallel()
	assert.Equal(t, Region("us-phoenix-1"), CodeToRegion("phx"))
	assert.Equal(t, Region("us-ashburn-1"), CodeToRegion("iad"))
	assert.Equal(t, Region(""), CodeToRegion("unknown"))
}

func TestResolveRegion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, in, want string
	}{
		{"short code expands", "phx", "us-phoenix-1"},
		{"short code uppercased", "PHX", "us-phoenix-1"},
		{"short code padded", "  iad  ", "us-ashburn-1"},
		// The passthrough cases guard existing usage: every config and
		// script in the wild supplies a full identifier.
		{"full identifier unchanged", "us-phoenix-1", "us-phoenix-1"},
		{"full identifier lowercased", "US-PHOENIX-1", "us-phoenix-1"},
		// Not in regionByShortName, but the dash marks it as an identifier
		// rather than a typo, so it must reach the loader intact.
		{"unmapped full identifier unchanged", "ap-westtokyo-1", "ap-westtokyo-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveRegion(tc.in)
			require.NoError(t, err)
			assert.Equal(t, Region(tc.want), got)
		})
	}
}

func TestResolveRegion_UnknownCodeIsAnError(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"zzz", "phoenix", ""} {
		_, err := ResolveRegion(in)
		require.Error(t, err, "input %q should not resolve", in)
		// The message must name the offending value and both accepted
		// forms; the whole point of erroring here is that the loader's
		// "environment is not valid or in the list" doesn't.
		assert.Contains(t, err.Error(), "unknown region code")
	}
}
