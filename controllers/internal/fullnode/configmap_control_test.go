package fullnode

import (
	"context"
	"errors"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestConfigMapControl_Reconcile(t *testing.T) {
	t.Parallel()

	type (
		mockConfigClient = mockClient[*corev1.ConfigMap]
		mockConfigDiffer = mockDiffer[*corev1.ConfigMap]
	)
	ctx := context.Background()

	t.Run("create", func(t *testing.T) {
		var mClient mockConfigClient
		mClient.GetObjectErr = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
		control := NewConfigMapControl(&mClient)
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Name = "stargaze"
		crd.Spec.ChainConfig.Network = "testnet"

		control.diffFactory = func(revisionLabelKey string, current, want []*corev1.ConfigMap) configmapDiffer {
			require.Equal(t, "app.kubernetes.io/revision", revisionLabelKey)
			require.Empty(t, current)
			require.Len(t, want, 3)
			return mockConfigDiffer{
				StubCreates: want,
			}
		}

		err := control.Reconcile(ctx, nopLogger, &crd, nil)
		require.NoError(t, err)

		require.Equal(t, 3, mClient.CreateCount)
		require.NotNil(t, mClient.LastCreateObject)
		require.Equal(t, "stargaze-testnet-fullnode-2", mClient.LastCreateObject.GetName())

		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)

		require.Zero(t, mClient.UpdateCount)
	})

	t.Run("updates", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "stargaze"
		crd.Spec.ChainConfig.Network = "testnet"

		var stubCm corev1.ConfigMap
		stubCm.Name = "stub"
		var mClient mockConfigClient
		mClient.ObjectList = corev1.ConfigMapList{Items: []corev1.ConfigMap{{}, {}}}

		control := NewConfigMapControl(&mClient)
		control.diffFactory = func(revisionLabelKey string, current, want []*corev1.ConfigMap) configmapDiffer {
			require.Len(t, current, 2)
			return mockConfigDiffer{
				StubUpdates: []*corev1.ConfigMap{{}},
			}
		}

		err := control.Reconcile(ctx, nopLogger, &crd, nil)
		require.NoError(t, err)

		require.Zero(t, mClient.CreateCount)
		require.Equal(t, 1, mClient.UpdateCount)
		require.NotNil(t, mClient.LastUpdateObject)

		require.Len(t, mClient.GotListOpts, 3)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, "test", listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, "app.kubernetes.io/name=stargaze-testnet-fullnode", listOpt.LabelSelector.String())
		require.Equal(t, ".metadata.controller=stargaze", listOpt.FieldSelector.String())
	})

	t.Run("deletes", func(t *testing.T) {
		crd := defaultCRD()
		crd.Name = "stargaze"
		crd.Spec.ChainConfig.Network = "testnet"

		var mClient mockConfigClient

		control := NewConfigMapControl(&mClient)
		control.diffFactory = func(revisionLabelKey string, current, want []*corev1.ConfigMap) configmapDiffer {
			return mockConfigDiffer{
				StubDeletes: []*corev1.ConfigMap{{}, {}, {}},
			}
		}

		err := control.Reconcile(ctx, nopLogger, &crd, nil)
		require.NoError(t, err)

		require.Zero(t, mClient.UpdateCount)
		require.Zero(t, mClient.CreateCount)
		require.Equal(t, 3, mClient.DeleteCount)

		require.Len(t, mClient.GotListOpts, 3)
	})

	t.Run("build error", func(t *testing.T) {
		var mClient mockConfigClient
		control := NewConfigMapControl(&mClient)
		control.build = func(crd *cosmosv1.CosmosFullNode, _ ExternalAddresses) ([]*corev1.ConfigMap, error) {
			return nil, errors.New("boom")
		}

		crd := defaultCRD()
		err := control.Reconcile(ctx, nopLogger, &crd, nil)

		require.Error(t, err)
		require.EqualError(t, err, "unrecoverable error: boom")
		require.False(t, err.IsTransient())
	})
}
