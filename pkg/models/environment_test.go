package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvironment_GettersAndEquals(t *testing.T) {
	t.Parallel()
	env := Environment{
		Type:   "dev",
		Region: "us-phoenix-1",
		Realm:  "oc1",
	}
	assert.Equal(t, "dev-phx", env.GetName())
	assert.ElementsMatch(t, []string{"dev", "us-phoenix-1", "oc1", "dev-phx"}, env.FilterableFields())
	assert.IsType(t, "", env.KubeContext())
	assert.False(t, env.IsFaulty())

	env2 := Environment{
		Type:   "dev",
		Region: "us-phoenix-1",
		Realm:  "oc1",
	}
	assert.True(t, env.Equals(env2))

	env3 := Environment{
		Type:   "prod",
		Region: "us-ashburn-1",
		Realm:  "oc1",
	}
	assert.False(t, env.Equals(env3))
}

func TestEnvironment_KubeContext(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		envType string
		region  string
		want    string
	}{
		{name: "dev", envType: "dev", region: "us-phoenix-1", want: "dp-dev-phx"},
		{name: "prod", envType: "prod", region: "us-ashburn-1", want: "dp-prod-iad"},
		// "preprod" is aliased to "ppe" in the context name.
		{name: "preprod aliased to ppe", envType: "preprod", region: "us-phoenix-1", want: "dp-ppe-phx"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			env := Environment{Type: c.envType, Region: c.region, Realm: "oc1"}
			assert.Equal(t, c.want, env.KubeContext())
		})
	}
}

func TestEnvironment_Equals_DifferentRealm(t *testing.T) {
	t.Parallel()
	// Same name, different realm: not equal.
	a := Environment{Type: "dev", Region: "us-phoenix-1", Realm: "oc1"}
	b := Environment{Type: "dev", Region: "us-phoenix-1", Realm: "oc2"}
	assert.False(t, a.Equals(b))
}
