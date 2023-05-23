package cosmos

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

type mockCollector struct {
}

func (m mockCollector) Collect(ctx context.Context, pods []corev1.Pod) StatusCollection {
	return nil
}

func TestStatusCache_Reconcile(t *testing.T) {
	t.Run("create, update, or patch", func(t *testing.T) {

	})

	t.Run("delete", func(t *testing.T) {

	})
}
