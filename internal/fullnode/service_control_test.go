package fullnode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceControl_Reconcile(t *testing.T) {
	t.Parallel()

	type mockSvcClient = mockClient[*corev1.Service]

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		crd := defaultCRD()
		crd.Namespace = "test"
		crd.Spec.Replicas = 3
		crd.Spec.Service.MaxP2PExternalAddresses = ptr(int32(2)) // Causes 1 p2p service to be created.

		var mClient mockSvcClient
		mClient.ObjectList = corev1.ServiceList{Items: []corev1.Service{
			{ObjectMeta: metav1.ObjectMeta{Name: "osmosis-p2p-0", Namespace: crd.Namespace}}, // update
			{ObjectMeta: metav1.ObjectMeta{Name: "osmosis-rpc", Namespace: crd.Namespace}},   // update
			{ObjectMeta: metav1.ObjectMeta{Name: "osmosis-99", Namespace: crd.Namespace}},    // Tests we never delete services.
		}}

		control := NewServiceControl(&mClient)
		err := control.Reconcile(ctx, nopReporter, &crd)
		require.NoError(t, err)

		require.Len(t, mClient.GotListOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, "test", listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, ".metadata.controller=osmosis", listOpt.FieldSelector.String())

		require.Equal(t, 2, mClient.CreateCount) // Created 2 p2p services.
		require.Equal(t, "osmosis-p2p-2", mClient.LastCreateObject.Name)
		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)

		require.Equal(t, 2, mClient.UpdateCount)
		require.Zero(t, mClient.DeleteCount) // Services are never deleted.
	})
}
