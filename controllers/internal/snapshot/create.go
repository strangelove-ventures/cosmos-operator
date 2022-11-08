package snapshot

import (
	"context"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateClient creates and sets owner reference.
type CreateClient interface {
	Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	Scheme() *runtime.Scheme
}

// Creator creates objects and assigns the owner reference.
type Creator[T client.Object] struct {
	builder func() ([]T, error)
	client  CreateClient
}

// NewCreator returns a valid Creator.
func NewCreator[T client.Object](client CreateClient, builder func() ([]T, error)) Creator[T] {
	return Creator[T]{
		builder: builder,
		client:  client,
	}
}

// Create builds the resources, creates them, and assigns owner reference.
func (c Creator[T]) Create(ctx context.Context, crd *cosmosv1.HostedSnapshot) error {
	resources, err := c.builder()
	if err != nil {
		return fmt.Errorf("build resources: %w", err)
	}

	logger := log.FromContext(ctx)
	for _, r := range resources {
		gk := r.GetObjectKind().GroupVersionKind().GroupKind().String()
		logger.Info("Creating resource", "groupKind", gk, "resource", r.GetName())
		if err = c.client.Create(ctx, r); err != nil {
			return err
		}
		err = ctrl.SetControllerReference(crd, r, c.client.Scheme())
		if err != nil {
			return err
		}
	}

	return nil
}
