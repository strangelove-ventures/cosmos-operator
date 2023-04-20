package fullnode

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"

	cmtjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PeerInfo contains information about a peer.
type PeerInfo struct {
	NodeID          p2p.ID
	PrivateAddress  string // Only the private address my-service.namespace.svc.cluster.local:<port>
	ExternalAddress string // Only the address <external-ip-or-hostname>:<port>. Not all peers will be external.

	hasExternalAddress bool
}

// PrivatePeer returns the full private identifier of the peer in the format <node_id>@<private_address>:<port>.
func (info PeerInfo) PrivatePeer() string {
	return string(info.NodeID) + "@" + info.PrivateAddress
}

// ExternalPeer returns the full external address of the peer in the format <node_id>@<external_address>:<port>.
func (info PeerInfo) ExternalPeer() string {
	if info.ExternalAddress == "" {
		return string(info.NodeID) + "@" + net.JoinHostPort("0.0.0.0", strconv.Itoa(p2pPort))
	}
	return string(info.NodeID) + "@" + info.ExternalAddress
}

// Peers maps an ObjectKey using the instance name to PeerInfo.
type Peers map[client.ObjectKey]PeerInfo

// HasIncompleteExternalAddress returns true if any peer has an external address but it is not assigned yet.
func (p Peers) HasIncompleteExternalAddress() bool {
	for _, peer := range p {
		if peer.hasExternalAddress && peer.ExternalAddress == "" {
			return true
		}
	}
	return false
}

// AllExternal returns a sorted list of all external peers in the format <node_id>@<external_address>:<port>.
func (p Peers) AllExternal() []string {
	addrs := lo.Map(lo.Values(p), func(info PeerInfo, _ int) string { return info.ExternalPeer() })
	sort.Strings(addrs)
	return addrs
}

// PeerCollector finds and collects peer information.
type PeerCollector struct {
	client Getter
}

func NewPeerCollector(client Getter) *PeerCollector {
	return &PeerCollector{client: client}
}

// Collect peer information given the crd.
func (c PeerCollector) Collect(ctx context.Context, crd *cosmosv1.CosmosFullNode) (Peers, kube.ReconcileError) {
	peers := make(Peers)
	for i := int32(0); i < crd.Spec.Replicas; i++ {
		secretName := nodeKeySecretName(crd, i)
		var secret corev1.Secret
		// Hoping the caching layer kubebuilder prevents API errors or rate limits. Simplifies logic to use a Get here
		// vs. manually filtering through a List.
		if err := c.client.Get(ctx, client.ObjectKey{Name: secretName, Namespace: crd.Namespace}, &secret); err != nil {
			return nil, kube.TransientError(fmt.Errorf("get secret %s: %w", secretName, err))
		}

		var nodeKey p2p.NodeKey
		if err := cmtjson.Unmarshal(secret.Data[nodeKeyFile], &nodeKey); err != nil {
			return nil, kube.UnrecoverableError(err)
		}
		svcName := p2pServiceName(crd, i)
		peers[c.objectKey(crd, i)] = PeerInfo{
			NodeID:         nodeKey.ID(),
			PrivateAddress: fmt.Sprintf("%s.%s.svc.cluster.local:%d", svcName, secret.Namespace, p2pPort),
		}
		if err := c.addExternalAddress(ctx, peers, crd, i); err != nil {
			return nil, kube.TransientError(err)
		}
	}

	return peers, nil
}

func (c PeerCollector) objectKey(crd *cosmosv1.CosmosFullNode, ordinal int32) client.ObjectKey {
	return client.ObjectKey{Name: instanceName(crd, ordinal), Namespace: crd.Namespace}
}

func (c PeerCollector) addExternalAddress(ctx context.Context, peers Peers, crd *cosmosv1.CosmosFullNode, ordinal int32) error {
	svcName := p2pServiceName(crd, ordinal)
	var svc corev1.Service
	// Hoping the caching layer kubebuilder prevents API errors or rate limits. Simplifies logic to use a Get here
	// vs. manually filtering through a List.
	if err := c.client.Get(ctx, client.ObjectKey{Name: svcName, Namespace: crd.Namespace}, &svc); err != nil {
		return kube.TransientError(fmt.Errorf("get server %s: %w", svcName, err))
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return nil
	}
	objKey := c.objectKey(crd, ordinal)
	info := peers[objKey]
	info.hasExternalAddress = true
	defer func() { peers[objKey] = info }()

	ingress := svc.Status.LoadBalancer.Ingress
	if len(ingress) == 0 {
		return nil
	}

	lb := ingress[0]
	host := lo.Ternary(lb.IP != "", lb.IP, lb.Hostname)
	if host != "" {
		info.ExternalAddress = net.JoinHostPort(host, strconv.Itoa(p2pPort))
	}
	return nil
}
