package cosmos

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TendermintStatuser calls the Tendermint RPC status endpoint.
type TendermintStatuser interface {
	Status(ctx context.Context, rpcHost string) (TendermintStatus, error)
}

// Lister can list resources, subset of client.Client.
type Lister interface {
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}

// PodCollector collects the tendermint/cometbft status of all pods owned by a controller.
type PodCollector struct {
	client     Lister
	tendermint TendermintStatuser
	timeout    time.Duration
}

// NewPodCollector returns a valid PodCollector. The timeout (if > 0) is used for each method call.
// Timeout is exposed here because it is important for good performance in reconcile loops,
// and reminds callers to set it.
// If unset, a default timeout is applied.
func NewPodCollector(client Lister, tendermint TendermintStatuser, timeout time.Duration) *PodCollector {
	return &PodCollector{client: client, tendermint: tendermint, timeout: timeout}
}

// Collect returns a PodCollection for the given controller. The controller must own the pods.
// Any non-nil error can be treated as transient and retried.
func (coll PodCollector) Collect(ctx context.Context, controller client.ObjectKey) (PodCollection, error) {
	var list corev1.PodList
	if err := coll.client.List(ctx, &list,
		client.InNamespace(controller.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: controller.Name},
	); err != nil {
		return nil, err
	}

	var (
		eg       errgroup.Group
		statuses = make([]Pod, len(list.Items))
	)

	for i := range list.Items {
		i := i
		eg.Go(func() error {
			pod := list.Items[i]
			statuses[i].pod = &pod
			ip := pod.Status.PodIP
			if ip == "" {
				// Check for IP, so we don't pay overhead of making a request.
				statuses[i].err = errors.New("pod has no IP")
				return nil
			}
			host := fmt.Sprintf("http://%s:26657", ip)
			cctx, cancel := context.WithTimeout(ctx, coll.timeoutDur())
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

func (coll PodCollector) timeoutDur() time.Duration {
	if coll.timeout <= 0 {
		return 30 * time.Second
	}
	return coll.timeout
}
