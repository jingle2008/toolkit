package config

import (
	"reflect"
	"testing"
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

// contains reports whether substr is in s.
func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && (s == substr || contains(s[1:], substr)))
}
