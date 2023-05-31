package cosmos

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
)

// Statuser calls the RPC status endpoint.
type Statuser interface {
	Status(ctx context.Context, rpcHost string) (CometStatus, error)
}

// StatusCollector collects the CometBFT status of all pods owned by a controller.
type StatusCollector struct {
	comet   Statuser
	timeout time.Duration
}

// NewStatusCollector returns a valid StatusCollector.
// Timeout is exposed here because it is important for good performance in reconcile loops,
// and reminds callers to set it.
func NewStatusCollector(comet Statuser, timeout time.Duration) *StatusCollector {
	return &StatusCollector{comet: comet, timeout: timeout}
}

// Collect returns a StatusCollection for the given pods.
// Any non-nil error can be treated as transient and retried.
func (coll StatusCollector) Collect(ctx context.Context, pods []corev1.Pod) StatusCollection {
	var eg errgroup.Group
	now := time.Now()
	statuses := make(StatusCollection, len(pods))

	for i := range pods {
		i := i
		eg.Go(func() error {
			pod := pods[i]
			statuses[i].Ts = now
			statuses[i].Pod = &pod
			ip := pod.Status.PodIP
			if ip == "" {
				// Check for IP, so we don't pay overhead of making a request.
				statuses[i].Err = errors.New("pod has no IP")
				return nil
			}
			host := fmt.Sprintf("http://%s:26657", ip)
			cctx, cancel := context.WithTimeout(ctx, coll.timeout)
			defer cancel()
			resp, err := coll.comet.Status(cctx, host)
			if err != nil {
				statuses[i].Err = err
				return nil
			}
			statuses[i].Status = resp
			return nil
		})
	}

	_ = eg.Wait()
	sort.Sort(statuses)
	return statuses
}
