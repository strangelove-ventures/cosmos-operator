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

var panicStatuser = mockStatuser(func(ctx context.Context, rpcHost string) (TendermintStatus, error) {
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

		tmClient := mockStatuser(func(ctx context.Context, rpcHost string) (TendermintStatus, error) {
			_, ok := ctx.Deadline()
			if !ok {
				require.Fail(t, "expected deadline in context")
			}
			var status TendermintStatus
			status.Result.NodeInfo.ListenAddr = rpcHost
			return status, nil
		})

		coll := NewStatusCollector(tmClient, timeout)
		got, err := coll.Collect(ctx, pods)
		require.NoError(t, err)

		require.Len(t, got, 3)

		for i, podStatus := range got {
			require.Equal(t, namespace, podStatus.Pod().Namespace)
			require.Equal(t, fmt.Sprintf("pod-%d", i), podStatus.Pod().Name)

			tmStatus, err := podStatus.Status()
			require.NoError(t, err)

			require.Equal(t, fmt.Sprintf("http://%d:26657", i), tmStatus.Result.NodeInfo.ListenAddr)
		}
	})

	t.Run("no pod IP", func(t *testing.T) {
		coll := NewStatusCollector(panicStatuser, timeout)
		got, err := coll.Collect(ctx, make([]corev1.Pod, 1))

		require.NoError(t, err)
		require.Len(t, got, 1)

		_, err = got[0].Status()
		require.Error(t, err)
		require.EqualError(t, err, "pod has no IP")
	})

	t.Run("status error", func(t *testing.T) {
		tmClient := mockStatuser(func(ctx context.Context, rpcHost string) (TendermintStatus, error) {
			return TendermintStatus{}, errors.New("status error")
		})
		coll := NewStatusCollector(tmClient, timeout)
		var pod corev1.Pod
		pod.Status.PodIP = "1.1.1.1"
		got, err := coll.Collect(ctx, []corev1.Pod{pod})

		require.NoError(t, err)
		require.Len(t, got, 1)

		_, err = got[0].Status()
		require.Error(t, err)
		require.EqualError(t, err, "status error")
	})

	t.Run("no pods", func(t *testing.T) {
		coll := NewStatusCollector(panicStatuser, timeout)
		got, err := coll.Collect(ctx, nil)

		require.NoError(t, err)
		require.Empty(t, got)
	})
}
