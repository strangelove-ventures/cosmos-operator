package cosmos

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

type mockStatuser func(ctx context.Context, rpcHost string) (CometStatus, error)

func (fn mockStatuser) Status(ctx context.Context, rpcHost string) (CometStatus, error) {
	return fn(ctx, rpcHost)
}

var panicStatuser = mockStatuser(func(ctx context.Context, rpcHost string) (CometStatus, error) {
	panic("should not be called")
})

func TestStatusCollector_Collect(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	const (
		namespace = "default"
		timeout   = time.Second
	)

	t.Run("happy path", func(t *testing.T) {
		pods := lo.Map(lo.Range(3), func(i int, _ int) corev1.Pod {
			pod := corev1.Pod{
				Status: corev1.PodStatus{
					PodIP: strconv.Itoa(i),
				},
			}
			pod.Name = fmt.Sprintf("pod-%d", i)
			pod.Namespace = namespace
			return pod
		})

		cometClient := mockStatuser(func(ctx context.Context, rpcHost string) (CometStatus, error) {
			_, ok := ctx.Deadline()
			if !ok {
				require.Fail(t, "expected deadline in context")
			}
			var status CometStatus
			status.Result.NodeInfo.ListenAddr = rpcHost
			return status, nil
		})

		coll := NewStatusCollector(cometClient, timeout)
		got := coll.Collect(ctx, pods)

		require.Len(t, got, 3)

		for i, podStatus := range got {
			require.Equal(t, namespace, podStatus.GetPod().Namespace)
			require.Equal(t, fmt.Sprintf("pod-%d", i), podStatus.GetPod().Name)

			tmStatus, err := podStatus.GetStatus()
			require.NoError(t, err)

			require.Equal(t, fmt.Sprintf("http://%d:26657", i), tmStatus.Result.NodeInfo.ListenAddr)
			require.WithinDuration(t, time.Now(), podStatus.Timestamp(), 10*time.Second)
		}

		timestamps := lo.Map(got, func(item StatusItem, _ int) time.Time { return item.Timestamp() })
		require.Len(t, lo.Uniq(timestamps), 1)
	})

	t.Run("no pod IP", func(t *testing.T) {
		coll := NewStatusCollector(panicStatuser, timeout)
		got := coll.Collect(ctx, make([]corev1.Pod, 1))

		require.Len(t, got, 1)

		_, err := got[0].GetStatus()
		require.Error(t, err)
		require.EqualError(t, err, "pod has no IP")
		require.NotZero(t, got[0].Timestamp())
	})

	t.Run("status error", func(t *testing.T) {
		cometClient := mockStatuser(func(ctx context.Context, rpcHost string) (CometStatus, error) {
			return CometStatus{}, errors.New("status error")
		})
		coll := NewStatusCollector(cometClient, timeout)
		var pod corev1.Pod
		pod.Status.PodIP = "1.1.1.1"
		got := coll.Collect(ctx, []corev1.Pod{pod})

		require.Len(t, got, 1)

		_, err := got[0].GetStatus()
		require.Error(t, err)
		require.EqualError(t, err, "status error")
		require.NotZero(t, got[0].Timestamp())
	})

	t.Run("no pods", func(t *testing.T) {
		coll := NewStatusCollector(panicStatuser, timeout)
		got := coll.Collect(ctx, nil)

		require.Empty(t, got)
	})
}
