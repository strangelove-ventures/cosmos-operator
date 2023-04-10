package fullnode

import (
	"context"
	"errors"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/cosmos"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockLister func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error

func (fn mockLister) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if ctx == nil {
		panic("context is nil")
	}
	return fn(ctx, list, opts...)
}

type mockStatuser func(ctx context.Context, rpcHost string) (cosmos.TendermintStatus, error)

func (fn mockStatuser) Status(ctx context.Context, rpcHost string) (cosmos.TendermintStatus, error) {
	return fn(ctx, rpcHost)
}

func TestPeerCollector_CollectAddresses(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	pods := []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "pod-0"}, Status: corev1.PodStatus{PodIP: "1.1.1.1"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}, Status: corev1.PodStatus{PodIP: "2.2.2.2"}},
	}
	var crd cosmosv1.CosmosFullNode
	crd.Name = "agoric"
	crd.Namespace = "test"

	t.Run("happy path", func(t *testing.T) {
		lister := mockLister(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
			list.(*corev1.PodList).Items = pods

			require.Len(t, opts, 2)
			var listOpt client.ListOptions
			for _, opt := range opts {
				opt.ApplyToList(&listOpt)
			}
			require.Equal(t, "test", listOpt.Namespace)
			require.Zero(t, listOpt.Limit)
			require.Equal(t, ".metadata.controller=agoric", listOpt.FieldSelector.String())
			return nil
		})

		statuser := mockStatuser(func(ctx context.Context, rpcHost string) (cosmos.TendermintStatus, error) {
			require.NotNil(t, ctx)
			var status cosmos.TendermintStatus

			switch rpcHost {
			case "http://1.1.1.1:26657":
				status.Result.NodeInfo.ID = "foo"
				// tcp:// added by tendermint rpc if external_address is blank
				status.Result.NodeInfo.ListenAddr = "tcp://0.0.0.0:26656"
				return status, nil
			case "http://2.2.2.2:26657":
				status.Result.NodeInfo.ID = "bar"
				status.Result.NodeInfo.ListenAddr = "12.34.56.78:26656"
				return status, nil
			default:
				panic("unexpected rpcHost: " + rpcHost)
			}
		})

		collector := NewPeerCollector(lister, statuser)

		got, err := collector.CollectAddresses(ctx, &crd)
		require.NoError(t, err)

		require.Equal(t, []string{"foo@0.0.0.0:26656", "bar@12.34.56.78:26656"}, got)
	})

	t.Run("tendermint error", func(t *testing.T) {
		lister := mockLister(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
			list.(*corev1.PodList).Items = pods
			return nil
		})

		statuser := mockStatuser(func(ctx context.Context, rpcHost string) (cosmos.TendermintStatus, error) {
			return cosmos.TendermintStatus{}, errors.New("tendermint error")
		})

		collector := NewPeerCollector(lister, statuser)

		_, err := collector.CollectAddresses(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "tendermint error")
	})

	t.Run("list error", func(t *testing.T) {
		lister := mockLister(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
			return errors.New("list error")
		})

		statuser := mockStatuser(func(ctx context.Context, rpcHost string) (cosmos.TendermintStatus, error) {
			panic("should not be called")
		})

		collector := NewPeerCollector(lister, statuser)
		_, err := collector.CollectAddresses(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "list error")
	})
}
