package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImportedModel_OwnerState(t *testing.T) {
	t.Parallel()
	var nilOwner ImportedModel
	assert.Equal(t, "", nilOwner.OwnerState(), "nil owner: want empty string")

	internal := ImportedModel{Owner: &Tenant{IsInternal: true}}
	assert.Equal(t, "true", internal.OwnerState(), "internal owner: want \"true\"")

	external := ImportedModel{Owner: &Tenant{IsInternal: false}}
	assert.Equal(t, "false", external.OwnerState(), "external owner: want \"false\"")
}

func TestImportedModel_FilterableFields(t *testing.T) {
	t.Parallel()
	m := ImportedModel{
		BaseModel: BaseModel{Name: "mymodel", DisplayName: "My Model", Status: "Ready"},
		Namespace: "ns-a",
		TenantID:  "t123",
	}

	fields := m.FilterableFields()
	// The imported-specific identity fields extend BaseModel's set.
	assert.Subset(t, fields, m.BaseModel.FilterableFields())
	assert.Contains(t, fields, "ns-a")
	assert.Contains(t, fields, "t123")
}

func TestImportedModel_TenancyOCID(t *testing.T) {
	t.Parallel()
	m := ImportedModel{TenantID: "t123"}
	assert.Equal(t, "ocid1.tenancy.oc1..t123", m.TenancyOCID("oc1"))

	// Orphans keep the placeholder suffix rather than producing an empty tail.
	orphan := ImportedModel{TenantID: "UNKNOWN_TENANCY"}
	assert.Equal(t, "ocid1.tenancy.oc1..UNKNOWN_TENANCY", orphan.TenancyOCID("oc1"))
}

func TestImportedModel_OCID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		region string
		want   string
	}{
		{
			name:   "IAD short code",
			region: string(RegionIAD),
			want:   "ocid1.generativeaiimportedmodel.oc1.iad.mymodel",
		},
		{
			name:   "PHX short code",
			region: string(RegionPHX),
			want:   "ocid1.generativeaiimportedmodel.oc1.phx.mymodel",
		},
		{
			name:   "other region passthrough",
			region: "eu-frankfurt-1",
			want:   "ocid1.generativeaiimportedmodel.oc1.eu-frankfurt-1.mymodel",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := ImportedModel{BaseModel: BaseModel{Name: "mymodel"}}
			assert.Equal(t, c.want, m.OCID("oc1", c.region))
		})
	}
}
