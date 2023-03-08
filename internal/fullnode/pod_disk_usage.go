package fullnode

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/healthcheck"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"go.uber.org/multierr"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DiskUsager fetches disk usage statistics
type DiskUsager interface {
	DiskUsage(ctx context.Context, host string) (healthcheck.DiskUsageResponse, error)
}

type PodDiskUsage struct {
	Name        string // pod name
	PercentUsed int
}

// CollectPodDiskUsage retrieves the disk usage information for all Pods belonging to the specified CosmosFullNode.
//
// It returns a slice of PodDiskUsage objects representing the disk usage information for each Pod or an error
// if fetching disk usage from all pods was unsuccessful.
func CollectPodDiskUsage(ctx context.Context, crd *cosmosv1.CosmosFullNode, lister Lister, diskClient DiskUsager) ([]PodDiskUsage, error) {
	var pods corev1.PodList
	if err := lister.List(ctx, &pods,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
	); err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, errors.New("no pods found")
	}

	var (
		found = make([]PodDiskUsage, len(pods.Items))
		errs  = make([]error, len(pods.Items))
		eg    errgroup.Group
	)

	for i := range pods.Items {
		i := i
		eg.Go(func() error {
			pod := pods.Items[i]
			cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			resp, err := diskClient.DiskUsage(cctx, "http://"+pod.Status.PodIP)
			if err != nil {
				errs[i] = fmt.Errorf("pod %s: %w", pod.Name, err)
				return nil
			}
			found[i].Name = pod.Name
			found[i].PercentUsed = int((float64(resp.AllBytes-resp.FreeBytes) / float64(resp.AllBytes)) * 100)
			return nil
		})
	}

	_ = eg.Wait()

	errs = lo.Filter(errs, func(item error, _ int) bool {
		return item != nil
	})
	if len(errs) == len(pods.Items) {
		return nil, multierr.Combine(errs...)
	}

	return lo.Compact(found), nil
}
