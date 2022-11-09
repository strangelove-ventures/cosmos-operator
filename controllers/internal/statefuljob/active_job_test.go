package statefuljob

import (
	"context"
	"errors"
	"testing"

	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockGetter func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error

func (fn mockGetter) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return fn(ctx, key, obj, opts...)
}

func TestFindActiveJob(t *testing.T) {
	t.Parallel()

	var (
		ctx = context.Background()
		crd cosmosalpha.StatefulJob
	)
	crd.Namespace = "test-ns"
	crd.Name = "test"

	var foundJob batchv1.Job
	foundJob.Name = "found-me"

	t.Run("happy path", func(t *testing.T) {
		getter := mockGetter(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			require.NotNil(t, ctx)
			require.Equal(t, "test-ns", key.Namespace)
			require.Equal(t, "snapshot-test", key.Name)
			require.Empty(t, opts)

			ref := obj.(*batchv1.Job)
			*ref = foundJob

			return nil
		})

		found, job, err := FindActiveJob(ctx, getter, &crd)

		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, foundJob, *job)
	})

	t.Run("not found", func(t *testing.T) {
		isNotFoundErr = func(err error) bool { return true }

		getter := mockGetter(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			return errors.New("stub not found")
		})

		found, _, err := FindActiveJob(ctx, getter, &crd)

		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("error", func(t *testing.T) {
		isNotFoundErr = kube.IsNotFound

		getter := mockGetter(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			return errors.New("boom")
		})

		_, _, err := FindActiveJob(ctx, getter, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "boom")
	})
}
