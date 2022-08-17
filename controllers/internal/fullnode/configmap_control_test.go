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
)

func TestConfigMapControl_Reconcile(t *testing.T) {
	t.Parallel()

	type configClient = mockClient[*corev1.ConfigMap]
	ctx := context.Background()

	t.Run("create", func(t *testing.T) {
		var mClient configClient
		mClient.GetObjectErr = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
		control := NewConfigMapControl(&mClient)
		crd := defaultCRD()
		crd.Name = "stargaze"
		crd.Spec.ChainConfig.Network = "testnet"

		requeue, err := control.Reconcile(ctx, nopLogger, &crd)
		require.NoError(t, err)
		require.False(t, requeue)

		require.NotNil(t, mClient.LastCreateObject)
		require.Equal(t, "stargaze-testnet-fullnode", mClient.LastCreateObject.GetName())

		require.NotEmpty(t, mClient.LastCreateObject.OwnerReferences)
		require.Equal(t, crd.Name, mClient.LastCreateObject.OwnerReferences[0].Name)
		require.Equal(t, "CosmosFullNode", mClient.LastCreateObject.OwnerReferences[0].Kind)
		require.True(t, *mClient.LastCreateObject.OwnerReferences[0].Controller)

		require.Nil(t, mClient.LastUpdateObject)
	})

	t.Run("updates", func(t *testing.T) {
		var mClient configClient
		control := NewConfigMapControl(&mClient)
		crd := defaultCRD()
		crd.Name = "stargaze"
		crd.Spec.ChainConfig.Network = "testnet"

		requeue, err := control.Reconcile(ctx, nopLogger, &crd)
		require.NoError(t, err)
		require.False(t, requeue)

		require.Nil(t, mClient.LastCreateObject)
		require.NotNil(t, mClient.LastUpdateObject)
		require.Equal(t, "stargaze-testnet-fullnode", mClient.LastUpdateObject.GetName())
	})

	t.Run("no-op", func(t *testing.T) {
		var mClient configClient
		control := NewConfigMapControl(&mClient)
		cm := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "config"},
			Data:       map[string]string{"test": "value", "another": "value"},
		}
		mClient.Object = cm
		control.build = func(crd *cosmosv1.CosmosFullNode) (corev1.ConfigMap, error) {
			return *cm.DeepCopy(), nil
		}

		crd := defaultCRD()
		requeue, err := control.Reconcile(ctx, nopLogger, &crd)
		require.NoError(t, err)
		require.False(t, requeue)

		require.Nil(t, mClient.LastCreateObject)
		require.Nil(t, mClient.LastUpdateObject)
	})

	t.Run("build error", func(t *testing.T) {
		var mClient configClient
		control := NewConfigMapControl(&mClient)
		control.build = func(crd *cosmosv1.CosmosFullNode) (corev1.ConfigMap, error) {
			return corev1.ConfigMap{}, errors.New("boom")
		}

		crd := defaultCRD()
		_, err := control.Reconcile(ctx, nopLogger, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "unrecoverable error: boom")
		require.False(t, err.IsTransient())
	})
}
