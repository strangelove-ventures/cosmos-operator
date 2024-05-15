package fullnode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	policyv1 "k8s.io/api/policy/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPodDisruptionBudgetControl_Reconcile(t *testing.T) {
	t.Parallel()

	type mockpdbClient = mockClient[*policyv1.PodDisruptionBudget]

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		crd := defaultCRD()
		crd.Namespace = "test"
		crd.Spec.Replicas = 2

		var mClient mockpdbClient

		control := NewPodDisruptionBudgetControl(&mClient)
		err := control.Reconcile(ctx, nopReporter, &crd)
		require.NoError(t, err)

		require.Len(t, mClient.GotListOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, "test", listOpt.Namespace)
		require.Zero(t, listOpt.Limit)

		require.Equal(t, 1, mClient.CreateCount) // Created 1 pdb
		require.Equal(t, "osmosis-vc-pdb", mClient.LastCreateObject.Name)
		require.Equal(t, "PodDisruptionBudget", mClient.LastCreateObject.Kind)
		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)
	})
}
