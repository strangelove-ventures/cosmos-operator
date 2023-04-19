package fullnode

import (
	"fmt"

	cmtjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PeerInfo struct {
	NodeID         string
	PrivateAddress string // The full address in the format <node_id>@my-service.namespace.svc.cluster.local:<port>
}

type PeerInfoCollection map[client.ObjectKey]PeerInfo

// BuildPeerInfo builds a PeerInfoCollection from a list of secrets.
// Secrets must be ordered by ordinal.
func BuildPeerInfo(secrets []*corev1.Secret, crd *cosmosv1.CosmosFullNode) (PeerInfoCollection, error) {
	peers := make(PeerInfoCollection)
	for i, secret := range secrets {
		var nodeKey p2p.NodeKey
		if err := cmtjson.Unmarshal(secret.Data[nodeKeyFile], &nodeKey); err != nil {
			return nil, err
		}
		instance := instanceName(crd, int32(i))
		svcName := p2pServiceName(crd, int32(i))
		key := client.ObjectKey{Name: instance, Namespace: secret.Namespace}
		peers[key] = PeerInfo{
			NodeID:         string(nodeKey.ID()),
			PrivateAddress: fmt.Sprintf("%s@%s.%s.svc.cluster.local:%d", nodeKey.ID(), svcName, secret.Namespace, p2pPort),
		}
	}
	return peers, nil
}
