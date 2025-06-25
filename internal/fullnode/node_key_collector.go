package fullnode

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type NodeKey struct {
	PrivKey NodeKeyPrivKey `json:"priv_key"`
}

type NodeKeyPrivKey struct {
	Type  string             `json:"type"`
	Value ed25519.PrivateKey `json:"value"`
}

func (nk NodeKey) ID() string {
	pub := nk.PrivKey.Value.Public()
	hash := sha256.Sum256(pub.(ed25519.PublicKey))
	return hex.EncodeToString(hash[:20])
}

// base64StrToNodeKey converts a base64-encoded ed25519 private key string into a NodeKey structure.
func base64StrToNodeKey(s string) (*NodeKey, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 string: %w", err)
	}

	if len(keyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid key size: expected %d bytes, got %d", ed25519.PrivateKeySize, len(keyBytes))
	}

	pk := ed25519.PrivateKey(keyBytes)

	return &NodeKey{
		PrivKey: NodeKeyPrivKey{
			Type:  "tendermint/PrivKeyEd25519",
			Value: pk,
		},
	}, nil
}

func randNodeKey() (*NodeKey, error) {
	_, pk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ed25519 node key: %w", err)
	}
	return &NodeKey{
		PrivKey: NodeKeyPrivKey{
			Type:  "tendermint/PrivKeyEd25519",
			Value: pk,
		},
	}, nil
}

// NodeKeyRepresenter represents a NodeKey and its marshaled form. Since NodeKeys can be pulled from ConfigMaps, we store the marshaled form to avoid re-marshaling during ConfigMap creation.
type NodeKeyRepresenter struct {
	NodeKey          NodeKey
	MarshaledNodeKey []byte
}

// Namespace maps an ObjectKey using the instance name to NodeKey.
type NodeKeys map[client.ObjectKey]NodeKeyRepresenter

// NodeKeyCollector finds and collects node key information.
type NodeKeyCollector struct {
	client Client
}

func NewNodeKeyCollector(client Client) *NodeKeyCollector {
	return &NodeKeyCollector{
		client: client,
	}
}

func hasNodeKeyAtIndex(nodeKeys []string, index int32) bool {
	return nodeKeys != nil && int(index) < len(nodeKeys) && nodeKeys[index] != ""
}

// Collect node key information given the crd.
func (c NodeKeyCollector) Collect(ctx context.Context, crd *cosmosv1.CosmosFullNode) (NodeKeys, kube.ReconcileError) {
	logger := log.FromContext(ctx)
	nodeKeys := make(NodeKeys)

	var cms corev1.ConfigMapList
	if err := c.client.List(ctx, &cms,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
	); err != nil {
		return nil, kube.TransientError(fmt.Errorf("list existing configmaps: %w", err))
	}

	currentCms := ptrSlice(cms.Items)

	for i := crd.Spec.Ordinals.Start; i < crd.Spec.Ordinals.Start+crd.Spec.Replicas; i++ {
		var confMap corev1.ConfigMap
		confMap.Name = instanceName(crd, i)
		confMap.Namespace = crd.Namespace
		confMap = *kube.FindOrDefaultCopy(currentCms, &confMap)

		nodeKeyContent := confMap.Data[nodeKeyFile]

		var nodeKey NodeKey
		var marshaledNodeKey []byte

		if nodeKeyContent != "" {
			err := json.Unmarshal([]byte(nodeKeyContent), &nodeKey)
			if err != nil {
				return nil, kube.UnrecoverableError(fmt.Errorf("unmarshal node key: %w", err))
			}

			// Store the exact value of the node key in the configmap to avoid non-deterministic JSON marshaling which can cause unnecessary updates.
			marshaledNodeKey = []byte(nodeKeyContent)
		} else if hasNodeKeyAtIndex(crd.Spec.NodeKeys, i-crd.Spec.Ordinals.Start) {
			rNodeKey, err := base64StrToNodeKey(crd.Spec.NodeKeys[i-crd.Spec.Ordinals.Start])
			if err != nil {
				return nil, kube.UnrecoverableError(fmt.Errorf("invalid node key: %w", err))
			}
			nodeKey = *rNodeKey
			marshaledNodeKey, err = json.Marshal(nodeKey)
			if err != nil {
				return nil, kube.UnrecoverableError(fmt.Errorf("marshal node key: %w", err))
			}
		} else {
			rNodeKey, err := randNodeKey()
			if err != nil {
				return nil, kube.UnrecoverableError(fmt.Errorf("generate node key: %w", err))
			}
			nodeKey = *rNodeKey

			marshaledNodeKey, err = json.Marshal(nodeKey)
			if err != nil {
				return nil, kube.UnrecoverableError(fmt.Errorf("marshal node key: %w", err))
			}
			logger.Info("Generating new node key", "ordinal", i)
		}

		nodeKeys[client.ObjectKey{Name: instanceName(crd, i), Namespace: crd.Namespace}] = NodeKeyRepresenter{
			NodeKey:          nodeKey,
			MarshaledNodeKey: marshaledNodeKey,
		}
	}
	return nodeKeys, nil
}
