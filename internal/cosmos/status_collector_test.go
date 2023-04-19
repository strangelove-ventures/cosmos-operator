package cosmos

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockLister func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error

func (fn mockLister) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if ctx == nil {
		panic("nil context")
	}
	return fn(ctx, list, opts...)
}

var panicStatuser = mockStatuser(func(ctx context.Context, rpcHost string) (TendermintStatus, error) {
	panic("should not be called")
})

func TestStatusCollector_Collect(t *testing.T) {
	ctx := context.Background()

	const namespace = "default"

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
			var status TendermintStatus
			status.Result.NodeInfo.ListenAddr = rpcHost
			return status, nil
		})
		lister := mockLister(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
			require.Len(t, opts, 2)
			var gotOpts client.ListOptions
			for _, opt := range opts {
				opt.ApplyToList(&gotOpts)
			}
			require.Equal(t, namespace, gotOpts.Namespace)
			require.Zero(t, gotOpts.Limit)
			require.Equal(t, ".metadata.controller=test", gotOpts.FieldSelector.String())

			list.(*corev1.PodList).Items = pods
			return nil
		})

		coll := NewStatusCollector(lister, tmClient)
		got, err := coll.Collect(ctx, client.ObjectKey{Name: "test", Namespace: namespace})
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
		lister := mockLister(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
			list.(*corev1.PodList).Items = make([]corev1.Pod, 1)
			return nil
		})
		coll := NewStatusCollector(lister, panicStatuser)
		got, err := coll.Collect(ctx, client.ObjectKey{})

		require.NoError(t, err)
		require.Len(t, got, 1)

		_, err = got[0].Status()
		require.Error(t, err)
		require.EqualError(t, err, "pod has no IP")
	})

	t.Run("status error", func(t *testing.T) {
		lister := mockLister(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
			var pod corev1.Pod
			pod.Status.PodIP = "1.1.1.1"
			list.(*corev1.PodList).Items = []corev1.Pod{pod}
			return nil
		})
		tmClient := mockStatuser(func(ctx context.Context, rpcHost string) (TendermintStatus, error) {
			return TendermintStatus{}, errors.New("status error")
		})
		coll := NewStatusCollector(lister, tmClient)
		got, err := coll.Collect(ctx, client.ObjectKey{})

		require.NoError(t, err)
		require.Len(t, got, 1)

		_, err = got[0].Status()
		require.Error(t, err)
		require.EqualError(t, err, "status error")
	})

	t.Run("list error", func(t *testing.T) {
		lister := mockLister(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
			return errors.New("list error")
		})
		coll := NewStatusCollector(lister, panicStatuser)
		_, err := coll.Collect(ctx, client.ObjectKey{})

		require.Error(t, err)
		require.EqualError(t, err, "list error")
	})

	t.Run("no pods", func(t *testing.T) {
		lister := mockLister(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
			return nil
		})
		coll := NewStatusCollector(lister, panicStatuser)
		_, err := coll.Collect(ctx, client.ObjectKey{})

		require.Error(t, err)
		require.EqualError(t, err, "no pods found")
	})
}
