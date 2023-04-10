package fullnode

import (
	"context"
	"fmt"
	"net/url"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/cosmos"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PeerCollector finds and collects
type PeerCollector struct {
	client   Lister
	tmClient cosmos.TendermintStatuser
}

func NewPeerCollector(client Lister, tmClient cosmos.TendermintStatuser) *PeerCollector {
	return &PeerCollector{
		client:   client,
		tmClient: tmClient,
	}
}

// CollectAddresses queries pods for their tendermint/cometbft peer addresses and returns them in <node_id>@<ip:port> format.
// Any error can be treated as transient and retried.
func (collector PeerCollector) CollectAddresses(ctx context.Context, crd *cosmosv1.CosmosFullNode) ([]string, error) {
	var pods corev1.PodList
	if err := collector.client.List(ctx, &pods,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
	); err != nil {
		return nil, err
	}

	addresses := make([]string, len(pods.Items))
	var eg errgroup.Group
	for i := range pods.Items {
		i := i
		eg.Go(func() error {
			pod := pods.Items[i]
			host := fmt.Sprintf("http://%s:%d", pod.Status.PodIP, rpcPort)
			status, err := collector.tmClient.Status(ctx, host)
			if err != nil {
				return err
			}

			addr := status.Result.NodeInfo.ListenAddr
			u, err := url.Parse(addr)
			if err == nil {
				addr = u.Host
			}
			addr = fmt.Sprintf("%s@%s", status.Result.NodeInfo.ID, addr)

			addresses[i] = addr
			return nil
		})
	}

	err := eg.Wait()
	return addresses, err
}
