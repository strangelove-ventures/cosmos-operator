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

	type (
		mockSvcClient = mockClient[*corev1.Service]
		mockSvcDiffer = mockDiffer[*corev1.Service]
	)

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		crd := defaultCRD()
		crd.Namespace = "test"
		crd.Spec.Replicas = 3

		var mClient mockSvcClient
		mClient.ObjectList = corev1.ServiceList{Items: make([]corev1.Service, 4)}

		control := NewServiceControl(&mClient)
		control.diffFactory = func(current, want []*corev1.Service) svcDiffer {
			require.Equal(t, 4, len(current))
			require.EqualValues(t, 2, len(want)) // 1 p2p service and the rpc service.

			return mockSvcDiffer{
				StubCreates: []*corev1.Service{{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}}},
				StubUpdates: ptrSlice(make([]corev1.Service, 2)),
				StubDeletes: ptrSlice(make([]corev1.Service, 3)),
			}
		}
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

		require.Equal(t, 1, mClient.CreateCount)
		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)

		require.Equal(t, 2, mClient.UpdateCount)
		require.Zero(t, mClient.DeleteCount) // Services are never deleted.
	})
}
