package kube

import (
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
)

func TestVersionCache(t *testing.T) {
	t.Parallel()

	cache := NewVersionCache()
	crd := &cosmosv1.CosmosFullNode{}
	crd.ResourceVersion = "1"
	crd.UID = "foo"
	crd.GetNamespace()

	require.True(t, cache.HasChanged(crd))

	cache.Update(crd)
	require.False(t, cache.HasChanged(crd))

	crd.ResourceVersion = "2"
	require.True(t, cache.HasChanged(crd))

	cache.Update(crd)
	require.False(t, cache.HasChanged(crd))

	crd.UID = "bar"
	require.True(t, cache.HasChanged(crd))

	cache.Update(crd)
	require.False(t, cache.HasChanged(crd))
}
