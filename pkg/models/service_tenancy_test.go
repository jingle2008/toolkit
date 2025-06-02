package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceTenancy_Getters(t *testing.T) {
	st := ServiceTenancy{
		Name:        "svc1",
		Regions:     []string{"us-phoenix-1", "us-ashburn-1"},
		Realm:       "oc1",
		HomeRegion:  "us-phoenix-1",
		Environment: "dev",
	}
	assert.Equal(t, "svc1", st.GetName())
	assert.ElementsMatch(t, []string{"us-phoenix-1", "us-ashburn-1", "svc1", "oc1", "us-phoenix-1", "dev"}, st.GetFilterableFields())
	expectedEnvs := []Environment{
		{Type: "dev", Region: "us-phoenix-1", Realm: "oc1"},
		{Type: "dev", Region: "us-ashburn-1", Realm: "oc1"},
	}
	assert.Equal(t, expectedEnvs, st.Environments())
	assert.Equal(t, "oc1-dev-svc1", st.GetKey())
}
