package fullnode

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	"github.com/bharvest-devops/cosmos-operator/internal/healthcheck"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockDiskUsager func(ctx context.Context, host, homeDir string) (healthcheck.DiskUsageResponse, error)

func (fn mockDiskUsager) DiskUsage(ctx context.Context, host, homeDir string) (healthcheck.DiskUsageResponse, error) {
	return fn(ctx, host, homeDir)
}

func TestCollectDiskUsage(t *testing.T) {
	t.Parallel()

	type mockReader = mockClient[*corev1.Pod]

	ctx := context.Background()

	const namespace = "default"

	var crd cosmosv1.CosmosFullNode
	crd.Name = "cosmoshub"
	crd.Namespace = namespace
	appConfig := cosmosv1.SDKAppConfig{}
	crd.Spec.ChainSpec.CosmosSDK = &appConfig

	builder := NewPodBuilder(&crd)
	validPods := lo.Map(lo.Range(3), func(_ int, index int) corev1.Pod {
		pod, err := builder.WithOrdinal(int32(index)).Build()
		if err != nil {
			panic(err)
		}
		pod.Status.PodIP = fmt.Sprintf("10.0.0.%d", index)
		return *pod
	})

	t.Run("happy path", func(t *testing.T) {
		var reader mockReader
		reader.ObjectList = corev1.PodList{Items: validPods}
		reader.Object = corev1.PersistentVolumeClaim{
			Status: corev1.PersistentVolumeClaimStatus{
				Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("500Gi")},
			},
		}

		diskClient := mockDiskUsager(func(ctx context.Context, host, homeDir string) (healthcheck.DiskUsageResponse, error) {
			if homeDir != "/home/operator/cosmos" {
				return healthcheck.DiskUsageResponse{}, fmt.Errorf("unexpected homeDir: %s", homeDir)
			}
			var free uint64
			switch host {
			case "http://10.0.0.0":
				free = 900
			case "http://10.0.0.1":
				free = 500
			case "http://10.0.0.2":
				free = 15 // Tests rounding up
			default:
				panic(fmt.Errorf("unknown host: %s", host))
			}
			return healthcheck.DiskUsageResponse{
				AllBytes:  1000,
				FreeBytes: free,
			}, nil
		})

		coll := NewDiskUsageCollector(diskClient, &reader)
		got, err := coll.CollectDiskUsage(ctx, &crd)

		require.NoError(t, err)
		require.Len(t, got, 3)

		require.Len(t, reader.GotListOpts, 2)
		var listOpt client.ListOptions
		for _, opt := range reader.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, "default", listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, ".metadata.controller=cosmoshub", listOpt.FieldSelector.String())

		require.Equal(t, namespace, reader.GetObjectKey.Namespace)
		require.Contains(t, []string{"pvc-cosmoshub-0", "pvc-cosmoshub-1", "pvc-cosmoshub-2"}, reader.GetObjectKey.Name)

		sort.Slice(got, func(i, j int) bool {
			return got[i].Name < got[j].Name
		})

		result := got[0]
		require.Equal(t, "pvc-cosmoshub-0", result.Name)
		require.Equal(t, 10, result.PercentUsed)
		require.Equal(t, resource.MustParse("500Gi"), result.Capacity)

		result = got[1]
		require.Equal(t, "pvc-cosmoshub-1", result.Name)
		require.Equal(t, 50, result.PercentUsed)
		require.Equal(t, resource.MustParse("500Gi"), result.Capacity)

		result = got[2]
		require.Equal(t, "pvc-cosmoshub-2", result.Name)
		require.Equal(t, 99, result.PercentUsed) // Tests rounding to be close to output of `df`
		require.Equal(t, resource.MustParse("500Gi"), result.Capacity)
	})

	t.Run("custom home dir", func(t *testing.T) {
		var reader mockReader
		reader.ObjectList = corev1.PodList{Items: validPods}
		reader.Object = corev1.PersistentVolumeClaim{
			Status: corev1.PersistentVolumeClaimStatus{
				Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("500Gi")},
			},
		}

		diskClient := mockDiskUsager(func(ctx context.Context, host, homeDir string) (healthcheck.DiskUsageResponse, error) {
			if homeDir != "/home/operator/.gaia" {
				return healthcheck.DiskUsageResponse{}, fmt.Errorf("unexpected homeDir: %s", homeDir)
			}
			return healthcheck.DiskUsageResponse{
				AllBytes:  1000,
				FreeBytes: 900,
			}, nil
		})

		coll := NewDiskUsageCollector(diskClient, &reader)

		ccrd := crd.DeepCopy()
		ccrd.Spec.ChainSpec.HomeDir = ".gaia"
		_, err := coll.CollectDiskUsage(ctx, ccrd)

		require.NoError(t, err)
	})

	t.Run("no pods found", func(t *testing.T) {
		var reader mockReader
		diskClient := mockDiskUsager(func(ctx context.Context, host, homeDir string) (healthcheck.DiskUsageResponse, error) {
			panic("should not be called")
		})

		coll := NewDiskUsageCollector(diskClient, &reader)
		_, err := coll.CollectDiskUsage(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "no pods found")
	})

	t.Run("list error", func(t *testing.T) {
		var reader mockReader
		reader.ObjectList = corev1.PodList{Items: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}, Status: corev1.PodStatus{PodIP: "10.0.0.1"}},
		}}
		reader.ListErr = errors.New("boom")
		diskClient := mockDiskUsager(func(ctx context.Context, host, homeDir string) (healthcheck.DiskUsageResponse, error) {
			panic("should not be called")
		})

		coll := NewDiskUsageCollector(diskClient, &reader)
		_, err := coll.CollectDiskUsage(ctx, &crd)

		require.Error(t, err)
		require.EqualError(t, err, "list pods: boom")
	})

	t.Run("partial disk client errors", func(t *testing.T) {
		var reader mockReader
		reader.ObjectList = corev1.PodList{Items: validPods}

		diskClient := mockDiskUsager(func(ctx context.Context, host, homeDir string) (healthcheck.DiskUsageResponse, error) {
			if host == "http://10.0.0.1" {
				return healthcheck.DiskUsageResponse{}, errors.New("boom")
			}
			return healthcheck.DiskUsageResponse{
				AllBytes:  100,
				FreeBytes: 100,
			}, nil
		})

		coll := NewDiskUsageCollector(diskClient, &reader)
		got, err := coll.CollectDiskUsage(ctx, &crd)

		require.NoError(t, err)
		require.Len(t, got, 2)

		gotNames := lo.Map(got, func(item PVCDiskUsage, _ int) string {
			return item.Name
		})
		require.NotContains(t, gotNames, "pvc-cosmoshub-1")
	})

	t.Run("disk client error", func(t *testing.T) {
		var reader mockReader
		reader.ObjectList = corev1.PodList{Items: []corev1.Pod{
			{ObjectMeta: metav1.ObjectMeta{Name: "1"}, Status: corev1.PodStatus{PodIP: "10.0.0.1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "2"}, Status: corev1.PodStatus{PodIP: "10.0.0.2"}},
		}}

		diskClient := mockDiskUsager(func(ctx context.Context, host, homeDir string) (healthcheck.DiskUsageResponse, error) {
			return healthcheck.DiskUsageResponse{Dir: "/some/dir"}, errors.New("boom")
		})

		var crd cosmosv1.CosmosFullNode

		coll := NewDiskUsageCollector(diskClient, &reader)
		_, err := coll.CollectDiskUsage(ctx, &crd)

		require.Error(t, err)
		require.Contains(t, err.Error(), "pod 1 /some/dir: boom")
		require.Contains(t, err.Error(), "pod 2 /some/dir: boom")
	})
}
