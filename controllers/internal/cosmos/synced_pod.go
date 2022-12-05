package cosmos

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
)

type TendermintStatuser interface {
	Status(ctx context.Context, rpcHost string) (TendermintStatus, error)
}

// SyncedPodFinder queries tendermint for block heights.
type SyncedPodFinder struct {
	tendermint TendermintStatuser
}

func NewSyncedPodFinder(status TendermintStatuser) *SyncedPodFinder {
	return &SyncedPodFinder{tendermint: status}
}

// SyncedPod returns the pod with the largest block height.
// If > 1 pod have the same largest height, returns the first pod with that height.
func (finder SyncedPodFinder) SyncedPod(ctx context.Context, candidates []*corev1.Pod) (*corev1.Pod, error) {
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
			cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			host := fmt.Sprintf("http://%s:26657", ip)
			resp, err := finder.tendermint.Status(cctx, host)
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
