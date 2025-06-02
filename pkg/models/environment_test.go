package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvironment_GettersAndEquals(t *testing.T) {
	env := Environment{
		Type:   "dev",
		Region: "us-phoenix-1",
		Realm:  "oc1",
	}
	assert.Equal(t, "dev-phx", env.GetName())
	assert.ElementsMatch(t, []string{"dev", "us-phoenix-1", "oc1", "dev-phx"}, env.GetFilterableFields())
	assert.Equal(t, "oc1-dev-phx", env.GetKey())
	assert.IsType(t, "", env.GetKubeContext())

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
