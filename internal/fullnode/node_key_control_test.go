package fullnode

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNodeKeyControl_Reconcile(t *testing.T) {
	t.Parallel()

	type mockNodeKeyClient = mockClient[*corev1.Secret]
	const namespace = "default"
	ctx := context.Background()

	var mClient mockNodeKeyClient
	var existing corev1.Secret
	existing.Name = "juno-node-key-0"
	existing.Namespace = namespace
	mClient.ObjectList = corev1.SecretList{Items: []corev1.Secret{existing}}

	crd := defaultCRD()
	crd.Namespace = namespace
	crd.Spec.Replicas = 3
	crd.Name = "juno"
	crd.Spec.ChainSpec.Network = "testnet"

	control := NewNodeKeyControl(&mClient)
	coll, err := control.Reconcile(ctx, nopReporter, &crd)
	require.NoError(t, err)

	require.Len(t, coll, 3)
	wantKeys := []client.ObjectKey{
		{Namespace: namespace, Name: "juno-0"},
		{Namespace: namespace, Name: "juno-1"},
		{Namespace: namespace, Name: "juno-2"},
	}
	require.ElementsMatch(t, wantKeys, lo.Keys(coll))
	require.Len(t, lo.Uniq(lo.Values(coll)), 3)
	for _, v := range coll {
		require.NotEmpty(t, v.NodeID)
	}

	require.Len(t, mClient.GotListOpts, 2)
	var listOpt client.ListOptions
	for _, opt := range mClient.GotListOpts {
		opt.ApplyToList(&listOpt)
	}
	require.Equal(t, namespace, listOpt.Namespace)
	require.Zero(t, listOpt.Limit)
	require.Equal(t, ".metadata.controller=juno", listOpt.FieldSelector.String())

	require.Equal(t, 1, mClient.UpdateCount)
	require.Equal(t, 2, mClient.CreateCount)

	require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
	require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
	require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
	require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)
}
