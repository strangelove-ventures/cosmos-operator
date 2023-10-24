package fullnode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRoleBindingControl_Reconcile(t *testing.T) {
	t.Parallel()

	type mockRbClient = mockClient[*rbacv1.RoleBinding]

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		crd := defaultCRD()
		crd.Namespace = "test"
		crd.Spec.Replicas = 3

		var mClient mockRbClient

		control := NewRoleBindingControl(&mClient)
		err := control.Reconcile(ctx, nopReporter, &crd)
		require.NoError(t, err)

		require.Len(t, mClient.GotListOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, "test", listOpt.Namespace)
		require.Zero(t, listOpt.Limit)

		require.Equal(t, 1, mClient.CreateCount) // Created 1 role binding.
		require.Equal(t, "osmosis-vc-rb", mClient.LastCreateObject.Name)
		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)

		mClient.ObjectList = rbacv1.RoleBindingList{Items: []rbacv1.RoleBinding{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "osmosis-vc-rb", Namespace: crd.Namespace},
				Subjects:   nil, // different to force update
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

		require.Equal(t, 1, mClient.UpdateCount) // Updated 1 role binding.

		require.Zero(t, mClient.DeleteCount) // Role bindings are never deleted.
	})
}
