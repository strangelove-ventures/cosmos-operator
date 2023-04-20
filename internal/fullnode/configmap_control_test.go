package fullnode

import (
	"context"
	"errors"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestConfigMapControl_Reconcile(t *testing.T) {
	t.Parallel()

	type mockConfigClient = mockClient[*corev1.ConfigMap]
	ctx := context.Background()
	const namespace = "test"

	t.Run("create", func(t *testing.T) {
		var mClient mockConfigClient
		mClient.ObjectList = corev1.ConfigMapList{Items: []corev1.ConfigMap{
			{ObjectMeta: metav1.ObjectMeta{Name: "stargaze-0", Namespace: namespace}},  // update
			{ObjectMeta: metav1.ObjectMeta{Name: "stargaze-1", Namespace: namespace}},  // update
			{ObjectMeta: metav1.ObjectMeta{Name: "stargaze-99", Namespace: namespace}}, // delete
		}}

		control := NewConfigMapControl(&mClient)
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Name = "stargaze"
		crd.Namespace = namespace
		crd.Spec.ChainSpec.Network = "testnet"

		cksums, err := control.Reconcile(ctx, nopReporter, &crd, nil)
		require.NoError(t, err)

		require.Len(t, mClient.GotListOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, "test", listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, ".metadata.controller=stargaze", listOpt.FieldSelector.String())

		require.Equal(t, 1, mClient.CreateCount)

		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)

		require.Equal(t, 2, mClient.UpdateCount)
		require.Equal(t, 1, mClient.DeleteCount)

		require.Len(t, cksums, 3)
		require.NotEmpty(t, cksums[client.ObjectKey{Name: "stargaze-0", Namespace: namespace}])
		require.NotEmpty(t, cksums[client.ObjectKey{Name: "stargaze-1", Namespace: namespace}])
		require.NotEmpty(t, cksums[client.ObjectKey{Name: "stargaze-2", Namespace: namespace}])
	})

	t.Run("build error", func(t *testing.T) {
		var mClient mockConfigClient
		control := NewConfigMapControl(&mClient)
		control.build = func(crd *cosmosv1.CosmosFullNode, _ Peers) ([]diff.Resource[*corev1.ConfigMap], error) {
			return nil, errors.New("boom")
		}

		crd := defaultCRD()
		_, err := control.Reconcile(ctx, nopReporter, &crd, nil)

		require.Error(t, err)
		require.EqualError(t, err, "boom")
		require.False(t, err.IsTransient())
	})
}
