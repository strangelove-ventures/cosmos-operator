package test

import (
	"testing"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
)

func HasTypeLabel(t *testing.T, builder func(crd cosmosv1.CosmosFullNode) []map[string]string) {
	t.Run("sets labels for", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Spec.Replicas = 3

		t.Run("type", func(t *testing.T) {
			t.Run("given unspecified type sets type to FullNode", func(t *testing.T) {
				resources := builder(crd)

				for _, resource := range resources {
					require.Equal(t, "FullNode", resource["cosmos.bharvest/type"])
				}
			})

			t.Run("given Sentry type", func(t *testing.T) {
				crd.Spec.Type = "Sentry"
				resources := builder(crd)

				for _, resource := range resources {
					require.Equal(t, "Sentry", resource["cosmos.bharvest/type"])
				}
			})

			t.Run("given FullNode type", func(t *testing.T) {
				crd.Spec.Type = "FullNode"
				resources := builder(crd)

				for _, resource := range resources {
					require.Equal(t, "FullNode", resource["cosmos.bharvest/type"])
				}
			})
		})
	})
}
