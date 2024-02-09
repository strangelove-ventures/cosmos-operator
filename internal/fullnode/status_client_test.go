package fullnode

import (
	"context"
	"errors"
	"testing"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type threadUnsafeClient struct {
	client.Client
	UpdateCount int
}

func (t *threadUnsafeClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return nil
}

func (t *threadUnsafeClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	t.UpdateCount++
	return nil
}

func (t *threadUnsafeClient) Status() client.StatusWriter {
	return t.SubResource("status")
}

type threadUnsafeSubResourceClient struct {
	client     *threadUnsafeClient
	subResouce string
}

func (t *threadUnsafeSubResourceClient) Get(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceGetOption) error {
	panic("threadUnsafeSubResourceClient does not support get")
}

func (t *threadUnsafeSubResourceClient) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	panic("threadUnsafeSubResourceClient does not support create")
}

func (t *threadUnsafeSubResourceClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	updateOptions := client.SubResourceUpdateOptions{}
	updateOptions.ApplyOptions(opts)

	body := obj
	if updateOptions.SubResourceBody != nil {
		body = updateOptions.SubResourceBody
	}

	return t.client.Update(ctx, body, &updateOptions)
}

func (t *threadUnsafeSubResourceClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	panic("threadUnsafeSubResourceClient does not support patch")
}

func (t *threadUnsafeClient) SubResource(subResource string) client.SubResourceClient {
	return &threadUnsafeSubResourceClient{client: t, subResouce: subResource}
}

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

	t.Run("concurrency", func(t *testing.T) {
		var mock threadUnsafeClient
		c := NewStatusClient(&mock)
		key := client.ObjectKey{Name: "test", Namespace: "default"}
		const total = 10
		var eg errgroup.Group
		for i := 0; i < total; i++ {
			eg.Go(func() error {
				return c.SyncUpdate(ctx, key, func(status *cosmosv1.FullNodeStatus) {})
			})
		}

		require.NoError(t, eg.Wait())
		require.Equal(t, 10, mock.UpdateCount)
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
