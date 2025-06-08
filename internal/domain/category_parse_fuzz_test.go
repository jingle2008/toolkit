package domain

import "testing"

func FuzzParseCategory(f *testing.F) {
	// Seed with known good and bad values
	seeds := []string{
		"Tenant", "tenant", "t", "LimitDefinition", "ld", "unknown", "", "   ", "BM", "basemodel", "dedicatedaicluster", "dac",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(_ *testing.T, input string) {
		_, _ = ParseCategory(input)
	})
}
