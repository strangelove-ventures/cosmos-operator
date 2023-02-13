package cosmos

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
)

// TendermintStatuser calls the Tendermint RPC status endpoint.
type TendermintStatuser interface {
	Status(ctx context.Context, rpcHost string) (TendermintStatus, error)
}

// PodFilter queries tendermint for block heights.
type PodFilter struct {
	tendermint TendermintStatuser
}

func NewPodFilter(status TendermintStatuser) *PodFilter {
	return &PodFilter{
		tendermint: status,
	}
}

// LargestHeight returns the pod with the largest block height regardless if catching up or not.
// If > 1 pod have the same largest height, returns the first pod with that height.
// Caller is responsible for timeouts via the context.
func (filter PodFilter) LargestHeight(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
	if len(candidates) == 0 {
		return nil, errors.New("missing candidates")
	}

	var (
		eg      errgroup.Group
		heights = make([]uint64, len(candidates))
	)

	for i := range candidates {
		i := i
		eg.Go(func() error {
			pod := candidates[i]
			ip := pod.Status.PodIP
			if ip == "" {
				return fmt.Errorf("pod %s: ip not assigned yet", pod.Name)
			}
			host := fmt.Sprintf("http://%s:26657", ip)
			resp, err := filter.tendermint.Status(ctx, host)
			if err != nil {
				return fmt.Errorf("pod %s: %w", pod.Name, err)
			}
			h := resp.LatestBlockHeight()
			if h == 0 {
				return fmt.Errorf("pod %s: tendermint status returned 0 for height", pod.Name)
			}
			heights[i] = h
			return nil
		})
	}

	err := eg.Wait()

	var (
		syncedIdx     int
		largestHeight uint64
	)
	for i, height := range heights {
		if height > largestHeight {
			largestHeight = height
			syncedIdx = i
		}
	}

	if largestHeight == 0 {
		return nil, err
	}

	return candidates[syncedIdx], nil
}

// SyncedPods returns all pods that are in sync (i.e. no longer catching up).
// Caller is responsible for timeouts via the context.
func (filter PodFilter) SyncedPods(ctx context.Context, log logr.Logger, candidates []*corev1.Pod) []*corev1.Pod {
	var (
		eg     errgroup.Group
		inSync = make([]*corev1.Pod, len(candidates))
	)

	for i := range candidates {
		i := i
		eg.Go(func() error {
			pod := candidates[i]
			logger := log.WithValues("pod", pod.Name)
			ip := pod.Status.PodIP
			if ip == "" {
				logger.Info("Pod has no IP")
				return nil
			}
			host := fmt.Sprintf("http://%s:26657", ip)
			resp, err := filter.tendermint.Status(ctx, host)
			if err != nil {
				logger.Error(err, "Failed to fetch tendermint rpc status")
				return nil
			}
			if resp.Result.SyncInfo.CatchingUp {
				logger.Info("Pod is still catching up")
				return nil
			}
			inSync[i] = pod
			return nil
		})
	}

	_ = eg.Wait()

	return lo.Compact(inSync)
}
