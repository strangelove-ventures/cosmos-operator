package cosmos

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

type mockStatuser func(ctx context.Context, rpcHost string) (TendermintStatus, error)

func (fn mockStatuser) Status(ctx context.Context, rpcHost string) (TendermintStatus, error) {
	return fn(ctx, rpcHost)
}

var nopLogger = logr.Discard()

func TestPodFilter_SyncedPods(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	pod1 := &corev1.Pod{
		Status: corev1.PodStatus{
			PodIP: "1",
		},
	}
	pod1.Name = "pod-1"
	pod2 := &corev1.Pod{
		Status: corev1.PodStatus{
			PodIP: "2",
		},
	}
	pod2.Name = "pod-2"

	t.Run("happy path", func(t *testing.T) {
		pod3 := pod1.DeepCopy()
		pod3.Name = "pod-3"
		pod3.Status.PodIP = "3"

		pod4 := pod1.DeepCopy()
		pod4.Name = "pod-4"
		pod4.Status.PodIP = "4"

		pod5 := pod1.DeepCopy()
		pod5.Name = "pod-5"
		pod5.Status.PodIP = "" // No IP yet

		client := mockStatuser(func(ctx context.Context, rpcHost string) (TendermintStatus, error) {
			var status TendermintStatus
			if ctx == nil {
				panic("nil context")
			}
			switch rpcHost {
			case "http://1:26657", "http://4:26657":
				status.Result.SyncInfo.CatchingUp = false
			case "http://2:26657":
				status.Result.SyncInfo.CatchingUp = true
			case "http://3:26657":
				status.Result.SyncInfo.LatestBlockHeight = "1000"
				return status, errors.New("filter me out")
			}
			return status, nil
		})
		filter := NewPodFilter(client)
		got := filter.SyncedPods(ctx, nopLogger, lo.Shuffle([]*corev1.Pod{pod1, pod2, pod3, pod4, pod5}))
		gotNames := lo.Map(got, func(p *corev1.Pod, _ int) string { return p.Name })

		require.Len(t, gotNames, 2)
		require.ElementsMatch(t, []string{"pod-1", "pod-4"}, gotNames)
	})

	t.Run("no candidates", func(t *testing.T) {
		client := mockStatuser(func(ctx context.Context, rpcHost string) (TendermintStatus, error) {
			panic("should not be called")
		})

		filter := NewPodFilter(client)
		got := filter.SyncedPods(ctx, nopLogger, nil)

		require.Empty(t, got)
	})
}
