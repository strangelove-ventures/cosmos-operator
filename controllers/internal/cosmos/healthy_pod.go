package cosmos

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

func SyncedPods(ctx context.Context, candidates []*corev1.Pod) ([]*corev1.Pod, error) {
	return nil, nil
}
