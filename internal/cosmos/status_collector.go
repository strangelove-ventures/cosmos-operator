package cosmos

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
)

// TendermintStatuser calls the Tendermint RPC status endpoint.
type TendermintStatuser interface {
	Status(ctx context.Context, rpcHost string) (TendermintStatus, error)
}

// StatusCollector collects the tendermint/cometbft status of all pods owned by a controller.
type StatusCollector struct {
	tendermint TendermintStatuser
	timeout    time.Duration
}

// NewStatusCollector returns a valid StatusCollector.
// Timeout is exposed here because it is important for good performance in reconcile loops,
// and reminds callers to set it.
func NewStatusCollector(tendermint TendermintStatuser, timeout time.Duration) *StatusCollector {
	return &StatusCollector{tendermint: tendermint, timeout: timeout}
}

// Collect returns a StatusCollection for the given pods.
// Any non-nil error can be treated as transient and retried.
func (coll StatusCollector) Collect(ctx context.Context, pods []corev1.Pod) (StatusCollection, error) {
	var eg errgroup.Group
	statuses := make(StatusCollection, len(pods))

	for i := range pods {
		i := i
		eg.Go(func() error {
			pod := pods[i]
			statuses[i].pod = &pod
			ip := pod.Status.PodIP
			if ip == "" {
				// Check for IP, so we don't pay overhead of making a request.
				statuses[i].err = errors.New("pod has no IP")
				return nil
			}
			host := fmt.Sprintf("http://%s:26657", ip)
			cctx, cancel := context.WithTimeout(ctx, coll.timeout)
			defer cancel()
			resp, err := coll.tendermint.Status(cctx, host)
			if err != nil {
				statuses[i].err = err
				return nil
			}
			statuses[i].status = resp
			return nil
		})
	}

	_ = eg.Wait()

	return statuses, nil
}

// SyncedPods returns all pods that are in sync (i.e. no longer catching up).
func (coll StatusCollector) SyncedPods(ctx context.Context, pods []corev1.Pod) ([]*corev1.Pod, error) {
	all, err := coll.Collect(ctx, pods)
	if err != nil {
		return nil, err
	}
	return all.SyncedPods(), nil
}
