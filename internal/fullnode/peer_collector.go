package fullnode

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"

	"github.com/samber/lo"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Peer contains information about a peer.
type Peer struct {
	NodeID          string
	PrivateAddress  string // Only the private address my-service.namespace.svc.cluster.local:<port>
	ExternalAddress string // Only the address <external-ip-or-hostname>:<port>. Not all peers will be external.

	hasExternalAddress bool
}

// PrivatePeer returns the full private identifier of the peer in the format <node_id>@<private_address>:<port>.
func (peer Peer) PrivatePeer() string {
	return peer.NodeID + "@" + peer.PrivateAddress
}

// ExternalPeer returns the full external address of the peer in the format <node_id>@<external_address>:<port>.
func (peer Peer) ExternalPeer() string {
	if peer.ExternalAddress == "" {
		return peer.NodeID + "@" + net.JoinHostPort("0.0.0.0", strconv.Itoa(p2pPort))
	}
	return peer.NodeID + "@" + peer.ExternalAddress
}

// Peers maps an ObjectKey using the instance name to Peer.
type Peers map[client.ObjectKey]Peer

func (peers Peers) Default() Peers { return make(Peers) }

// Get is a convenience getter.
func (peers Peers) Get(name, namespace string) Peer {
	if peers == nil {
		return Peer{}
	}
	return peers[client.ObjectKey{Name: name, Namespace: namespace}]
}

// Except returns a copy of the peers without the Peer for the given name and namespace.
func (peers Peers) Except(name, namespace string) Peers {
	peerCopy := make(Peers)
	objKey := client.ObjectKey{Name: name, Namespace: namespace}
	for key, peer := range peers {
		if key != objKey {
			peerCopy[key] = peer
		}
	}
	return peerCopy
}

// HasIncompleteExternalAddress returns true if any peer has an external address but it is not assigned yet.
func (peers Peers) HasIncompleteExternalAddress() bool {
	for _, peer := range peers {
		if peer.hasExternalAddress && peer.ExternalAddress == "" {
			return true
		}
	}
	return false
}

// NodeIDs returns a sorted list of all node IDs.
func (peers Peers) NodeIDs() []string {
	ids := lo.Map(lo.Values(peers), func(p Peer, _ int) string { return p.NodeID })
	sort.Strings(ids)
	return ids
}

// AllExternal returns a sorted list of all external peers in the format <node_id>@<external_address>:<port>.
func (peers Peers) AllExternal() []string {
	addrs := lo.Map(lo.Values(peers), func(info Peer, _ int) string { return info.ExternalPeer() })
	sort.Strings(addrs)
	return addrs
}

// AllPrivate returns a sorted list of all private peers in the format <node_id>@<private_address>:<port>.
func (peers Peers) AllPrivate() []string {
	addrs := lo.Map(lo.Values(peers), func(info Peer, _ int) string { return info.PrivatePeer() })
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
func (c PeerCollector) Collect(ctx context.Context, crd *cosmosv1.CosmosFullNode, nodeKeys NodeKeys) (Peers, kube.ReconcileError) {
	peers := make(Peers)
	startOrdinal := crd.Spec.Ordinals.Start

	clusterDomain := "cluster.local"
	if crd.Spec.Service.ClusterDomain != nil {
		clusterDomain = *crd.Spec.Service.ClusterDomain
	}

	for i := startOrdinal; i < startOrdinal+crd.Spec.Replicas; i++ {

		nodeKey, ok := nodeKeys[c.objectKey(crd, i)]

		if !ok {
			return nil, kube.UnrecoverableError(fmt.Errorf("node key not found for %s", c.objectKey(crd, i)))
		}

		svcName := p2pServiceName(crd, i)
		peers[c.objectKey(crd, i)] = Peer{
			NodeID:         nodeKey.NodeKey.ID(),
			PrivateAddress: fmt.Sprintf("%s.%s.svc.%s:%d", svcName, crd.Namespace, clusterDomain, p2pPort),
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
