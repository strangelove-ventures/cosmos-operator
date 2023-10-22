package fullnode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceAccountControl_Reconcile(t *testing.T) {
	t.Parallel()

	type mockSaClient = mockClient[*corev1.ServiceAccount]

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		crd := defaultCRD()
		crd.Namespace = "test"
		crd.Spec.Replicas = 3

		var mClient mockSaClient

		control := NewServiceAccountControl(&mClient)
		err := control.Reconcile(ctx, nopReporter, &crd)
		require.NoError(t, err)

		require.Len(t, mClient.GotListOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, "test", listOpt.Namespace)
		require.Zero(t, listOpt.Limit)

		require.Equal(t, 1, mClient.CreateCount) // Created 1 service account.
		require.Equal(t, "osmosis-vc-sa", mClient.LastCreateObject.Name)
		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)

		mClient.ObjectList = corev1.ServiceAccountList{Items: []corev1.ServiceAccount{
			{
				ObjectMeta:                   metav1.ObjectMeta{Name: "osmosis-vc-sa", Namespace: crd.Namespace},
				AutomountServiceAccountToken: ptr(true), // added to force update
			},
		}}

		mClient.GotListOpts = nil // reset for next reconcile

		err = control.Reconcile(ctx, nopReporter, &crd)
		require.NoError(t, err)

		require.Len(t, mClient.GotListOpts, 2)
		listOpt = client.ListOptions{}
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, "test", listOpt.Namespace)
		require.Zero(t, listOpt.Limit)

		require.Equal(t, 1, mClient.UpdateCount) // Updated 1 service account.

		require.Zero(t, mClient.DeleteCount) // Service accounts are never deleted.
	})
}
