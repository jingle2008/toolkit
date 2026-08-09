package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestConfig_Validate(t *testing.T) {
	t.Parallel()
	valid := Config{
		RepoPath:   "repo",
		KubeConfig: "kube",
		EnvType:    "type",
		EnvRegion:  "region",
		EnvRealm:   "realm",
		Category:   "Tenant",
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}

	// Each required field missing
	fields := []string{"RepoPath", "KubeConfig", "EnvType", "EnvRegion", "EnvRealm", "Category"}
	for _, f := range fields {
		cfg := valid
		reflect.ValueOf(&cfg).Elem().FieldByName(f).SetString("")
		err := cfg.Validate()
		if err == nil {
			t.Errorf("expected error for missing %s, got nil", f)
		}
	}

	// Invalid category
	cfg := valid
	cfg.Category = "notacategory"
	err := cfg.Validate()
	if err == nil || !contains(err.Error(), "unknown category") {
		t.Errorf("expected invalid category error, got: %v", err)
	}
}

func TestConfig_Normalize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                 string
		inType, inRegion     string
		wantType, wantRegion string
	}{
		{"code and ppe expand", "ppe", "phx", "preprod", "us-phoenix-1"},
		{"canonical values untouched", "prod", "us-ashburn-1", "prod", "us-ashburn-1"},
		{"case folded", "PPE", "IAD", "preprod", "us-ashburn-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := Config{EnvType: tc.inType, EnvRegion: tc.inRegion}
			if err := cfg.Normalize(); err != nil {
				t.Fatalf("Normalize: %v", err)
			}
			if cfg.EnvType != tc.wantType {
				t.Errorf("EnvType = %q, want %q", cfg.EnvType, tc.wantType)
			}
			if cfg.EnvRegion != tc.wantRegion {
				t.Errorf("EnvRegion = %q, want %q", cfg.EnvRegion, tc.wantRegion)
			}
		})
	}
}

// TestConfig_NormalizeLeavesEmptyEmpty guards the interaction with the
// required-setting checks: if Normalize resolved "" it would report an
// unknown region code for a value the user never supplied, hiding the
// "missing --env-region" message that actually tells them what to do.
func TestConfig_NormalizeLeavesEmptyEmpty(t *testing.T) {
	t.Parallel()
	cfg := Config{}
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize on empty config: %v", err)
	}
	if cfg.EnvType != "" || cfg.EnvRegion != "" {
		t.Errorf("empty values changed: type=%q region=%q", cfg.EnvType, cfg.EnvRegion)
	}
}

func TestConfig_NormalizeRejectsUnknownCode(t *testing.T) {
	t.Parallel()
	cfg := Config{EnvRegion: "zzz"}
	err := cfg.Normalize()
	if err == nil {
		t.Fatal("expected an error for region code \"zzz\"")
	}
	// The flag name must be in the message: the user typed --env-region,
	// not the mapstructure key or the Go field. Uses strings.Contains,
	// not the local contains helper below — that one only matches
	// suffixes, and the flag name sits mid-message.
	if !strings.Contains(err.Error(), "--env-region") {
		t.Errorf("error should name the flag, got: %v", err)
	}
}

/*
TestConfig_NormalizeFeedsCorrectOCID is the regression test for the one
silent-corruption path in this feature: DedicatedAICluster.OCID embeds
the region string verbatim, so an unexpanded short code yields an OCID
that looks plausible and addresses nothing.

Note the choice of region. OCID construction runs the region through
normalizeRegion, which maps us-ashburn-1 and us-phoenix-1 *back* to
their codes — so an iad/phx test would pass whether or not Normalize
ran and prove nothing. eu-frankfurt-1 keeps its full identifier, which
is what makes the difference observable.
*/
func TestConfig_NormalizeFeedsCorrectOCID(t *testing.T) {
	t.Parallel()
	dac := models.DedicatedAICluster{Name: "my-dac"}

	cfg := Config{EnvRealm: "oc1", EnvRegion: "fra"}
	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	want := dac.OCID("oc1", "eu-frankfurt-1")
	if got := dac.OCID(cfg.EnvRealm, cfg.EnvRegion); got != want {
		t.Errorf("OCID from normalized config = %q, want %q", got, want)
	}
	// And the un-normalized form really is different, so the assertion
	// above is not vacuous.
	if same := dac.OCID("oc1", "fra"); same == want {
		t.Fatal("raw code produces the same OCID — pick a region where it differs")
	}
}

// contains reports whether substr is in s.
func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && (s == substr || contains(s[1:], substr)))
}
