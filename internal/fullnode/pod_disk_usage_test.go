package fullnode

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/healthcheck"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockDiskUsager func(ctx context.Context, host string) (healthcheck.DiskUsageResponse, error)

func (fn mockDiskUsager) DiskUsage(ctx context.Context, host string) (healthcheck.DiskUsageResponse, error) {
	return fn(ctx, host)
}

func TestFindPodsDiskUsage(t *testing.T) {
	t.Parallel()

	type mockLister = mockClient[*corev1.Pod]

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		var lister mockLister
		lister.ObjectList = corev1.PodList{Items: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}, Status: corev1.PodStatus{PodIP: "10.0.0.1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-2"}, Status: corev1.PodStatus{PodIP: "10.0.0.2"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-3"}, Status: corev1.PodStatus{PodIP: "10.0.0.3"}},
		}}

		diskClient := mockDiskUsager(func(ctx context.Context, host string) (healthcheck.DiskUsageResponse, error) {
			var free uint64
			switch host {
			case "http://10.0.0.1":
				free = 90
			case "http://10.0.0.2":
				free = 50
			case "http://10.0.0.3":
				free = 10
			default:
				panic(fmt.Errorf("unkonwn host: %s", host))
			}
			return healthcheck.DiskUsageResponse{
				AllBytes:  100,
				FreeBytes: free,
			}, nil
		})

		var crd cosmosv1.CosmosFullNode
		crd.Name = "cosmoshub"
		crd.Namespace = "default"

		got, err := FindPodsDiskUsage(ctx, &crd, &lister, diskClient)

		require.NoError(t, err)
		require.Len(t, got, 3)

		require.Len(t, lister.GotListOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range lister.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, "default", listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, ".metadata.controller=cosmoshub", listOpt.FieldSelector.String())

		sort.Slice(got, func(i, j int) bool {
			return got[i].Name < got[j].Name
		})

		result := got[0]
		require.Equal(t, "pod-1", result.Name)
		require.Equal(t, 10, result.PercentUsed)

		result = got[1]
		require.Equal(t, "pod-2", result.Name)
		require.Equal(t, 50, result.PercentUsed)

		result = got[2]
		require.Equal(t, "pod-3", result.Name)
		require.Equal(t, 90, result.PercentUsed)
	})

	t.Run("no pods found", func(t *testing.T) {
		var lister mockLister
		diskClient := mockDiskUsager(func(ctx context.Context, host string) (healthcheck.DiskUsageResponse, error) {
			panic("should not be called")
		})

		var crd cosmosv1.CosmosFullNode
		_, err := FindPodsDiskUsage(ctx, &crd, &lister, diskClient)

		require.Error(t, err)
		require.EqualError(t, err, "no pods found")
	})

	t.Run("list error", func(t *testing.T) {
		var lister mockLister
		lister.ObjectList = corev1.PodList{Items: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}, Status: corev1.PodStatus{PodIP: "10.0.0.1"}},
		}}
		lister.ListErr = errors.New("boom")
		diskClient := mockDiskUsager(func(ctx context.Context, host string) (healthcheck.DiskUsageResponse, error) {
			panic("should not be called")
		})

		var crd cosmosv1.CosmosFullNode
		_, err := FindPodsDiskUsage(ctx, &crd, &lister, diskClient)

		require.Error(t, err)
		require.EqualError(t, err, "list pods: boom")
	})

	t.Run("partial disk client errors", func(t *testing.T) {
		var lister mockLister
		lister.ObjectList = corev1.PodList{Items: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}, Status: corev1.PodStatus{PodIP: "10.0.0.1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-2"}, Status: corev1.PodStatus{PodIP: "10.0.0.2"}},
		}}

		diskClient := mockDiskUsager(func(ctx context.Context, host string) (healthcheck.DiskUsageResponse, error) {
			if host == "http://10.0.0.1" {
				return healthcheck.DiskUsageResponse{}, errors.New("boom")
			}
			return healthcheck.DiskUsageResponse{
				AllBytes:  100,
				FreeBytes: 100,
			}, nil
		})

		var crd cosmosv1.CosmosFullNode

		got, err := FindPodsDiskUsage(ctx, &crd, &lister, diskClient)

		require.NoError(t, err)
		require.Len(t, got, 1)

		require.Equal(t, "pod-2", got[0].Name)
	})

	t.Run("disk client error", func(t *testing.T) {
		var lister mockLister
		lister.ObjectList = corev1.PodList{Items: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "1"}, Status: corev1.PodStatus{PodIP: "10.0.0.1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "2"}, Status: corev1.PodStatus{PodIP: "10.0.0.2"}},
		}}

		diskClient := mockDiskUsager(func(ctx context.Context, host string) (healthcheck.DiskUsageResponse, error) {
			return healthcheck.DiskUsageResponse{}, errors.New("boom")
		})

		var crd cosmosv1.CosmosFullNode

		_, err := FindPodsDiskUsage(ctx, &crd, &lister, diskClient)

		require.Error(t, err)
		require.Contains(t, err.Error(), "pod 1: boom")
		require.Contains(t, err.Error(), "pod 2: boom")
	})
}
