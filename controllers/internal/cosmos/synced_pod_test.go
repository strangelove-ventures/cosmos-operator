package cosmos

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

type mockStatuser func(ctx context.Context, rpcHost string) (TendermintStatus, error)

func (fn mockStatuser) Status(ctx context.Context, rpcHost string) (TendermintStatus, error) {
	return fn(ctx, rpcHost)
}

func TestSyncedPod(t *testing.T) {
	t.Parallel()
	rand.Seed(time.Now().UnixNano())

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
		errorPod := pod1.DeepCopy()
		errorPod.Name = "should-not-see-me"
		errorPod.Status.PodIP = "3"

		client := mockStatuser(func(ctx context.Context, rpcHost string) (TendermintStatus, error) {
			var status TendermintStatus
			if ctx == nil {
				panic("nil context")
			}
			switch rpcHost {
			case "http://1:26657":
				status.Result.SyncInfo.LatestBlockHeight = "1"
			case "http://2:26657":
				status.Result.SyncInfo.LatestBlockHeight = "2"
			case "http://3:26657":
				status.Result.SyncInfo.LatestBlockHeight = "1000"
				return status, errors.New("filter me out")
			default:
				panic(fmt.Errorf("unexpected host: %v", rpcHost))
			}
			return status, nil
		})

		got, err := SyncedPod(ctx, client, lo.Shuffle([]*corev1.Pod{pod1, pod2, errorPod}))

		require.NoError(t, err)
		require.Equal(t, "pod-2", got.Name)
	})

	t.Run("errors", func(t *testing.T) {
		client := mockStatuser(func(ctx context.Context, rpcHost string) (TendermintStatus, error) {
			var status TendermintStatus
			switch rpcHost {
			case "http://1:26657":
				status.Result.SyncInfo.LatestBlockHeight = "0"
			case "http://2:26657":
				return status, errors.New("boom")
			default:
				panic(fmt.Errorf("unexpected host: %v", rpcHost))
			}
			return status, nil
		})

		_, err := SyncedPod(ctx, client, lo.Shuffle([]*corev1.Pod{pod1, pod2}))

		require.Error(t, err)
	})

	t.Run("no candidates", func(t *testing.T) {
		client := mockStatuser(func(ctx context.Context, rpcHost string) (TendermintStatus, error) {
			panic("should not be called")
		})

		_, err := SyncedPod(ctx, client, nil)

		require.Error(t, err)
		require.EqualError(t, err, "missing candidates")
	})
}
