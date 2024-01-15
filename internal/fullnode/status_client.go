package fullnode

import (
	"context"
	"sync"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type semaphore chan struct{}

func newSem() semaphore {
	return make(semaphore, 1)
}

func (s semaphore) Acquire() {
	s <- struct{}{}
}

func (s semaphore) Release() {
	<-s
}

type StatusClient struct {
	sems   sync.Map
	client client.Client
}

func NewStatusClient(c client.Client) *StatusClient {
	return &StatusClient{client: c}
}

// SyncUpdate synchronizes updates to a CosmosFullNode's status subresource per client.ObjectKey.
// There are several controllers that update a fullnode's status to signal the fullnode controller to take action
// and update the cluster state.
//
// This method minimizes accidentally overwriting status fields by several actors.
//
// Server-side-apply, in theory, would be a solution. During testing, however, it resulted in many conflict errors
// and would require non-trivial migration to clear existing deployment's metadata.managedFields.
func (client *StatusClient) SyncUpdate(ctx context.Context, key client.ObjectKey, update func(status *cosmosv1.FullNodeStatus)) error {
	sem, _ := client.sems.LoadOrStore(key, newSem())
	sem.(semaphore).Acquire()
	defer sem.(semaphore).Release()

	var crd cosmosv1.CosmosFullNode
	if err := client.client.Get(ctx, key, &crd); err != nil {
		return err
	}

	update(&crd.Status)

	return client.client.Status().Update(ctx, &crd)
}
