package fullnode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceControl_Reconcile(t *testing.T) {
	t.Parallel()

	type (
		mockSvcClient = mockClient[*corev1.Service]
		mockSvcDiffer = mockDiffer[*corev1.Service]
	)

	ctx := context.Background()
	const namespace = "test"

	t.Run("create", func(t *testing.T) {
		crd := defaultCRD()
		crd.Namespace = "test"

		var mClient mockSvcClient
		control := NewServiceControl(&mClient)
		control.diffFactory = func(revisionLabelKey string, current, want []*corev1.Service) svcDiffer {
			require.Equal(t, "app.kubernetes.io/revision", revisionLabelKey)
			require.Empty(t, current)
			require.Len(t, want, 1)
			return mockSvcDiffer{
				StubCreates: want,
			}
		}
		_, err := control.Reconcile(ctx, nopLogger, &crd)
		require.NoError(t, err)

		require.Equal(t, 1, mClient.CreateCount)
		require.Equal(t, "osmosis-mainnet-fullnode-p2p", mClient.LastCreateObject.Name)

		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)
	})

	t.Run("update", func(t *testing.T) {
		crd := defaultCRD()
		crd.Namespace = "test"

		var stubSvc corev1.Service
		stubSvc.Name = "osmosis-mainnet-fullnode-p2p"
		stubSvc.Status = corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "4.5.6.7", Hostname: "lb.example.com"},
				},
			},
		}
		var mClient mockSvcClient
		mClient.ObjectList = corev1.ServiceList{Items: []corev1.Service{stubSvc, {}, {}}}

		control := NewServiceControl(&mClient)
		control.diffFactory = func(revisionLabelKey string, current, want []*corev1.Service) svcDiffer {
			require.Len(t, current, 3)
			var svc corev1.Service
			svc.Name = "stub-update"
			return mockSvcDiffer{StubUpdates: []*corev1.Service{&svc}}
		}
		result, err := control.Reconcile(ctx, nopLogger, &crd)
		require.NoError(t, err)

		require.Equal(t, "4.5.6.7", result.P2PIPAddress)
		require.Equal(t, "lb.example.com", result.P2PHostname)

		require.Zero(t, mClient.CreateCount)
		require.Equal(t, mClient.UpdateCount, 1)
		require.Equal(t, "stub-update", mClient.LastUpdateObject.Name)
	})

	t.Run("no changes", func(t *testing.T) {
		crd := defaultCRD()
		crd.Namespace = "test"

		var mClient mockSvcClient
		control := NewServiceControl(&mClient)
		control.diffFactory = func(revisionLabelKey string, current, want []*corev1.Service) svcDiffer {
			return mockSvcDiffer{}
		}
		_, err := control.Reconcile(ctx, nopLogger, &crd)
		require.NoError(t, err)

		require.Zero(t, mClient.CreateCount)
		require.Zero(t, mClient.UpdateCount)

		require.Len(t, mClient.GotListOpts, 3)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, namespace, listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, "app.kubernetes.io/name=osmosis-mainnet-fullnode", listOpt.LabelSelector.String())
		require.Equal(t, ".metadata.controller=osmosis", listOpt.FieldSelector.String())
	})
}

func TestServiceResult_P2PExternalAddress(t *testing.T) {
	var result ServiceResult
	require.Zero(t, result.P2PExternalAddress())

	result.P2PIPAddress = "1.2.3.4"
	require.Equal(t, "1.2.3.4:26656", result.P2PExternalAddress())

	result.P2PHostname = "test.com"
	require.Equal(t, "1.2.3.4:26656", result.P2PExternalAddress())

	result.P2PIPAddress = ""
	require.Equal(t, "test.com:26656", result.P2PExternalAddress())
}
