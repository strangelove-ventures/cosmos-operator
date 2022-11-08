package snapshot

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type mockDeleter func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error

func (fn mockDeleter) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if ctx == nil {
		panic("nil context")
	}
	if len(opts) > 0 {
		panic("expected 0 opts")
	}
	return fn(ctx, obj, opts...)
}

func TestDeletePairedPVC(t *testing.T) {
	ctx := context.Background()

	pred := DeletePVCPredicate(ctx, mockDeleter(func(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
		pvc := obj.(*corev1.PersistentVolumeClaim)
		require.Equal(t, "test", pvc.Namespace)
		require.Equal(t, "delete-me", pvc.Name)
		return nil
	}))

	var job batchv1.Job
	job.Namespace = "test"
	job.Name = "delete-me"
	job.Labels = defaultLabels()

	result := pred.Delete(event.DeleteEvent{Object: &job})
	require.True(t, result)
}
