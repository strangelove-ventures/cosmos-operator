package fullnode

import (
	"context"
	"errors"
	"fmt"
	"math"
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

type PVCDiskUsage struct {
	Name        string // pvc name
	PercentUsed int
}

type DiskUsageCollector struct {
	diskClient DiskUsager
	lister     Lister
}

func NewDiskUsageCollector(diskClient DiskUsager, lister Lister) *DiskUsageCollector {
	return &DiskUsageCollector{diskClient: diskClient, lister: lister}
}

// CollectDiskUsage retrieves the disk usage information for all pods belonging to the specified CosmosFullNode.
//
// It returns a slice of PVCDiskUsage objects representing the disk usage information for each PVC or an error
// if fetching disk usage via all pods was unsuccessful.
func (c DiskUsageCollector) CollectDiskUsage(ctx context.Context, crd *cosmosv1.CosmosFullNode) ([]PVCDiskUsage, error) {
	var pods corev1.PodList
	if err := c.lister.List(ctx, &pods,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
	); err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, errors.New("no pods found")
	}

	var (
		found = make([]PVCDiskUsage, len(pods.Items))
		errs  = make([]error, len(pods.Items))
		eg    errgroup.Group
	)

	for i := range pods.Items {
		i := i
		eg.Go(func() error {
			pod := pods.Items[i]
			cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			resp, err := c.diskClient.DiskUsage(cctx, "http://"+pod.Status.PodIP)
			if err != nil {
				errs[i] = fmt.Errorf("pod %s: %w", pod.Name, err)
				return nil
			}
			found[i].Name = PVCName(&pod)
			n := (float64(resp.AllBytes-resp.FreeBytes) / float64(resp.AllBytes)) * 100
			n = math.Round(n)
			found[i].PercentUsed = int(n)
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
