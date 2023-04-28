package cosmos

import (
	"context"
	"errors"
	"fmt"

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

// StatusCollector collects the tendermint/cometbft status of all pods owned by a controller.
type StatusCollector struct {
	client     Lister
	tendermint TendermintStatuser
}

func NewStatusCollector(client Lister, tendermint TendermintStatuser) *StatusCollector {
	return &StatusCollector{client: client, tendermint: tendermint}
}

// Collect returns a StatusCollection for the given controller. The controller must own the pods.
// Any non-nil error can be treated as transient and retried.
// Caller should pass a context with a reasonable timeout.
func (coll StatusCollector) Collect(ctx context.Context, controller client.ObjectKey) (StatusCollection, error) {
	var list corev1.PodList
	if err := coll.client.List(ctx, &list,
		client.InNamespace(controller.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: controller.Name},
	); err != nil {
		return nil, err
	}

	var (
		eg       errgroup.Group
		statuses = make([]PodStatus, len(list.Items))
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
			resp, err := coll.tendermint.Status(ctx, host)
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
