package fullnode

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Reader lists and gets objects.
type Reader = client.Reader

// Lister can list resources, subset of client.Client.
type Lister interface {
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}

type Getter interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
}

// StatusPatcher patches the status subresource of a resource.
type StatusPatcher interface {
	Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error
}
