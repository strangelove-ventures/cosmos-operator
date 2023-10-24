package fullnode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRoleControl_Reconcile(t *testing.T) {
	t.Parallel()

	type mockRoleClient = mockClient[*rbacv1.Role]

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		crd := defaultCRD()
		crd.Namespace = "test"
		crd.Spec.Replicas = 3

		var mClient mockRoleClient

		control := NewRoleControl(&mClient)
		err := control.Reconcile(ctx, nopReporter, &crd)
		require.NoError(t, err)

		require.Len(t, mClient.GotListOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, "test", listOpt.Namespace)
		require.Zero(t, listOpt.Limit)

		require.Equal(t, 1, mClient.CreateCount) // Created 1 role.
		require.Equal(t, "osmosis-vc-r", mClient.LastCreateObject.Name)
		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)

		mClient.ObjectList = rbacv1.RoleList{Items: []rbacv1.Role{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "osmosis-vc-r", Namespace: crd.Namespace},
				Rules:      nil, // added to force update
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

		require.Equal(t, 1, mClient.UpdateCount) // Updated 1 role.

		require.Zero(t, mClient.DeleteCount) // Roles are never deleted.
	})
}
