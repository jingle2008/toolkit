package collections

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestPickByName(t *testing.T) {
	t.Parallel()
	items := []models.GPUPool{{Name: "a"}, {Name: "b"}}
	got, err := PickByName(items, "b", "GPUPool")
	require.NoError(t, err)
	assert.Equal(t, "b", got.Name)

	_, err = PickByName(items, "B", "GPUPool")
	var none *NoSuchNameError
	require.ErrorAs(t, err, &none, "exact, case-sensitive")
	assert.EqualError(t, err, `no GPUPool named "B"`)

	_, err = PickByName([]models.GPUPool{{Name: "a"}, {Name: "a"}}, "a", "GPUPool")
	assert.EqualError(t, err, `2 GPUPool rows are named "a"`)
}

// Service tenancies share names across realms; <realm>/<name> picks one.
func TestPickByName_Qualified(t *testing.T) {
	t.Parallel()
	items := []models.ServiceTenancy{
		{Name: "prod", Realm: "oc4"}, {Name: "prod", Realm: "oc1"}, {Name: "dev", Realm: "oc1"},
	}
	_, err := PickByName(items, "prod", "ServiceTenancy")
	require.EqualError(t, err, `2 ServiceTenancy rows are named "prod": oc1/prod, oc4/prod; name one as listed`)

	got, err := PickByName(items, "oc4/prod", "ServiceTenancy")
	require.NoError(t, err)
	assert.Equal(t, "oc4", got.Realm)

	got, err = PickByName(items, "dev", "ServiceTenancy")
	require.NoError(t, err)
	assert.Equal(t, "oc1", got.Realm)

	_, err = PickByName(items, "oc2/prod", "ServiceTenancy")
	require.ErrorAs(t, err, new(*NoSuchNameError))
}

// A name shared across groups is ambiguous; <group>/<name> picks one.
func TestPickByNameGrouped(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.InferenceService{
		"ns-b": {{Name: "svc", Namespace: "ns-b"}},
		"ns-a": {{Name: "svc", Namespace: "ns-a"}, {Name: "other", Namespace: "ns-a"}},
	}
	_, _, err := PickByNameGrouped(grouped, "svc", "InferenceService")
	require.EqualError(t, err, `2 InferenceService rows are named "svc": ns-a/svc, ns-b/svc; name one as <group>/<name>`)
	assert.False(t, errors.As(err, new(*NoSuchNameError)))

	group, it, err := PickByNameGrouped(grouped, "ns-b/svc", "InferenceService")
	require.NoError(t, err)
	assert.Equal(t, "ns-b", group)
	assert.Equal(t, "ns-b", it.Namespace)

	group, it, err = PickByNameGrouped(grouped, "other", "InferenceService")
	require.NoError(t, err)
	assert.Equal(t, "ns-a", group)
	assert.Equal(t, "other", it.Name)

	_, _, err = PickByNameGrouped(grouped, "ns-c/svc", "InferenceService")
	require.ErrorAs(t, err, new(*NoSuchNameError))
}

// Rows sharing their group and name (one artifact built for two GPU
// configs) are listed, and picked, as <group>/<name>@<variant>: the form
// the error offers must be one that works.
func TestPickByNameGroupedVariant(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.ModelArtifact{
		"cmd-a": {
			{Name: "art", ModelName: "cmd-a", GPUCount: 2, GPUShape: "H200"},
			{Name: "art", ModelName: "cmd-a", GPUCount: 2, GPUShape: "H100"},
		},
	}
	for _, name := range []string{"art", "cmd-a/art"} {
		_, _, err := PickByNameGrouped(grouped, name, "ModelArtifact")
		require.EqualError(t, err, `2 ModelArtifact rows are named "`+name+`": cmd-a/art@2xH100, cmd-a/art@2xH200; name one as listed`)
	}

	group, it, err := PickByNameGrouped(grouped, "cmd-a/art@2xH200", "ModelArtifact")
	require.NoError(t, err)
	assert.Equal(t, "cmd-a", group)
	assert.Equal(t, "H200", it.GPUShape)

	_, _, err = PickByNameGrouped(grouped, "cmd-a/art@8xA10", "ModelArtifact")
	require.ErrorAs(t, err, new(*NoSuchNameError))
}
