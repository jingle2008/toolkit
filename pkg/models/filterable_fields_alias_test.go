package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFilterableFields_DoNotAliasModelSlices locks in that FilterableFields
// never appends into a model's own slice: with spare capacity (as JSON
// decoding leaves), an append would write into the shared backing array
// and race with any concurrent filter over the same row.
func TestFilterableFields_DoNotAliasModelSlices(t *testing.T) {
	t.Parallel()

	cases := map[string]func(s []string) []string{
		"BaseModel": func(s []string) []string {
			return BaseModel{Name: "m", Capabilities: s}.FilterableFields()
		},
		"ConsolePropertyRegionalOverride": func(s []string) []string {
			return ConsolePropertyRegionalOverride{Name: "p", Regions: s}.FilterableFields()
		},
		"ConsolePropertyTenancyOverride": func(s []string) []string {
			return ConsolePropertyTenancyOverride{
				TenantID:                        "t",
				ConsolePropertyRegionalOverride: ConsolePropertyRegionalOverride{Name: "p", Regions: s},
			}.FilterableFields()
		},
		"LimitRegionalOverride": func(s []string) []string {
			return LimitRegionalOverride{Name: "l", Regions: s}.FilterableFields()
		},
		"LimitTenancyOverride": func(s []string) []string {
			return LimitTenancyOverride{
				TenantID:              "t",
				LimitRegionalOverride: LimitRegionalOverride{Name: "l", Regions: s},
			}.FilterableFields()
		},
		"PropertyRegionalOverride": func(s []string) []string {
			return PropertyRegionalOverride{Name: "p", Regions: s}.FilterableFields()
		},
		"PropertyTenancyOverride": func(s []string) []string {
			return PropertyTenancyOverride{
				TenantID:                 "t",
				PropertyRegionalOverride: PropertyRegionalOverride{Name: "p", Regions: s},
			}.FilterableFields()
		},
		"ServiceTenancy": func(s []string) []string {
			return ServiceTenancy{Name: "s", Regions: s}.FilterableFields()
		},
	}

	for name, fields := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			backing := make([]string, 1, 8)
			backing[0] = "x"

			got := fields(backing)

			assert.Contains(t, got, "x")
			assert.Equal(t, make([]string, cap(backing)-1), backing[:cap(backing)][1:],
				"FilterableFields wrote into the model's backing array")
		})
	}
}
