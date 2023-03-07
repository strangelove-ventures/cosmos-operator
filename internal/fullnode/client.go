package fullnode

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Lister can list resources, subset of client.Client.
type Lister interface {
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}
