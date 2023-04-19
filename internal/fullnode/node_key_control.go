package fullnode

import (
	"context"
	"fmt"

	cmtjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NodeKeyControl reconciles node keys for a CosmosFullNode. Node keys are saved as secrets and later mounted
// into pods.
type NodeKeyControl struct {
	client Client
}

func NewNodeKeyControl(client Client) NodeKeyControl {
	return NodeKeyControl{
		client: client,
	}
}

// P2PInfo contains information about peer-to-peer.
type P2PInfo struct {
	NodeID string
}

// P2PInfoCollection is a map of instance names to its P2PInfo.
type P2PInfoCollection map[client.ObjectKey]P2PInfo

// Reconcile is the control loop for node keys. The secrets are never deleted.
func (control NodeKeyControl) Reconcile(ctx context.Context, reporter kube.Reporter, crd *cosmosv1.CosmosFullNode) (P2PInfoCollection, kube.ReconcileError) {
	var secrets corev1.SecretList
	if err := control.client.List(ctx, &secrets,
		client.InNamespace(crd.Namespace),
		client.MatchingFields{kube.ControllerOwnerField: crd.Name},
	); err != nil {
		return nil, kube.TransientError(fmt.Errorf("list existing node key secrets: %w", err))
	}

	existing := ptrSlice(secrets.Items)
	want, serr := BuildNodeKeySecrets(existing, crd)
	if serr != nil {
		return nil, kube.UnrecoverableError(fmt.Errorf("build node key secrets: %w", serr))
	}

	diffed := diff.New(existing, want)

	for _, secret := range diffed.Creates() {
		reporter.Info("Creating node key secret", "secret", secret.Name)
		if err := ctrl.SetControllerReference(crd, secret, control.client.Scheme()); err != nil {
			return nil, kube.TransientError(fmt.Errorf("set controller reference on node key secret %q: %w", secret.Name, err))
		}
		if err := control.client.Create(ctx, secret); kube.IgnoreAlreadyExists(err) != nil {
			return nil, kube.TransientError(fmt.Errorf("create node key secret %q: %w", secret.Name, err))
		}
	}

	for _, secret := range diffed.Updates() {
		reporter.Info("Updating node key secret", "secret", secret.Name)
		if err := control.client.Update(ctx, secret); err != nil {
			return nil, kube.TransientError(fmt.Errorf("update node key secret %q: %w", secret.Name, err))
		}
	}

	return control.p2pCollection(want)
}

func (control NodeKeyControl) p2pCollection(secrets []diff.Resource[*corev1.Secret]) (P2PInfoCollection, kube.ReconcileError) {
	coll := make(P2PInfoCollection)
	for _, res := range secrets {
		secret := res.Object()
		key := client.ObjectKey{Namespace: secret.Namespace, Name: secret.Labels[kube.InstanceLabel]}
		b := secret.Data[nodeKeyFile]
		var nodeKey p2p.NodeKey
		if err := cmtjson.Unmarshal(b, &nodeKey); err != nil {
			return nil, kube.UnrecoverableError(fmt.Errorf("unmarshal node key %s: %w", secret.Name, err))
		}
		coll[key] = P2PInfo{
			NodeID: string(nodeKey.ID()),
		}
	}
	return coll, nil
}
