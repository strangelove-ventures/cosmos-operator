package fullnode

import (
	"context"
	"errors"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestStatusClient_SyncUpdate(t *testing.T) {
	type mClient = mockClient[*cosmosv1.CosmosFullNode]

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		var (
			mock    mClient
			stubCRD cosmosv1.CosmosFullNode
		)
		stubCRD.Status.Phase = "test-phase"
		stubCRD.Name = "test"
		stubCRD.Namespace = "default"
		mock.Object = stubCRD

		c := NewStatusClient(&mock)
		key := client.ObjectKey{Name: "test", Namespace: "default"}
		msg := ptr("Here's test message")
		err := c.SyncUpdate(ctx, key, func(status *cosmosv1.FullNodeStatus) {
			status.StatusMessage = msg
		})

		require.NoError(t, err)

		require.Equal(t, key, mock.GetObjectKey)
		require.Equal(t, 1, mock.UpdateCount)

		updated := mock.LastUpdateObject
		want := stubCRD.DeepCopy()
		want.Status.StatusMessage = msg
		require.Equal(t, want.ObjectMeta, updated.ObjectMeta)
		require.Equal(t, want.Status, updated.Status)
	})

	t.Run("get error", func(t *testing.T) {
		var (
			mock mClient
		)
		mock.GetObjectErr = errors.New("get boom")

		c := NewStatusClient(&mock)
		key := client.ObjectKey{Name: "test", Namespace: "default"}
		err := c.SyncUpdate(ctx, key, nil)

		require.Error(t, err)
		require.EqualError(t, err, "get boom")
		require.Nil(t, mock.LastUpdateObject)
	})

	t.Run("update error", func(t *testing.T) {
		var (
			mock    mClient
			stubCRD cosmosv1.CosmosFullNode
		)
		mock.Object = stubCRD
		mock.UpdateErr = errors.New("update boom")

		c := NewStatusClient(&mock)
		key := client.ObjectKey{Name: "test", Namespace: "default"}
		err := c.SyncUpdate(ctx, key, func(status *cosmosv1.FullNodeStatus) {})

		require.Error(t, err)
		require.EqualError(t, err, "update boom")
	})
}
