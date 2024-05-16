package kube

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateOrUpdate first attempts to create the obj. If it already exists, it makes a second
// call to update the obj.
func CreateOrUpdate(ctx context.Context, client client.Writer, obj client.Object) error {
	err := client.Create(ctx, obj)
	if IsAlreadyExists(err) {
		err = client.Update(ctx, obj)
	}
	return err
}

// Create create the obj. If it already exists, do not update.
func Create(ctx context.Context, client client.Writer, obj client.Object) error {
	err := client.Create(ctx, obj)
	if IsAlreadyExists(err) {
		return nil
	}
	return err
}
