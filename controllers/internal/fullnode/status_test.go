package fullnode

import (
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
)

func TestResetStatus(t *testing.T) {
	t.Parallel()

	t.Run("basic happy path", func(t *testing.T) {
		var crd cosmosv1.CosmosFullNode
		crd.Generation = 123
		crd.Status.StatusMessage = ptr("should not see me")
		crd.Status.Phase = "should not see me"
		ResetStatus(&crd)

		require.EqualValues(t, 123, crd.Status.ObservedGeneration)
		require.Nil(t, crd.Status.StatusMessage)
		require.Equal(t, cosmosv1.FullNodePhaseProgressing, crd.Status.Phase)
	})
}
