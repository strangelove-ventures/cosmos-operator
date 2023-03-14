package fullnode

import (
	"context"
	"fmt"
	"sync"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StatusClient struct {
	mu sync.Mutex
	// TODO: use constructor
	client.Client
}

func (client *StatusClient) SyncUpdate(ctx context.Context, key client.ObjectKey, update func(status *cosmosv1.FullNodeStatus)) error {
	client.mu.Lock()
	defer client.mu.Unlock()

	var crd cosmosv1.CosmosFullNode
	if err := client.Get(ctx, key, &crd); err != nil {
		return fmt.Errorf("get %v: %w", key, err)
	}
	update(&crd.Status)
	return client.Status().Update(ctx, &crd)
}
