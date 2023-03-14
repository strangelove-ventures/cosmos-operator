package fullnode

import (
	"context"
	"fmt"
	"sync"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StatusService struct {
	mu sync.Mutex
	// TODO: use constructor
	client.Client
}

func (svc *StatusService) Update(ctx context.Context, key client.ObjectKey, update func(status *cosmosv1.FullNodeStatus)) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	var crd cosmosv1.CosmosFullNode
	if err := svc.Get(ctx, key, &crd); err != nil {
		return fmt.Errorf("get %v: %w", key, err)
	}
	update(&crd.Status)
	return svc.Status().Update(ctx, &crd)
}
