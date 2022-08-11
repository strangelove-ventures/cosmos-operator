package fullnode

import (
	"context"
	"fmt"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockClient[T client.Object] struct {
	ObjectList  any
	GotListOpts []client.ListOption

	CreateCount         int
	LastCreatedResource T

	DeleteCount int

	PatchCount      int
	LastPatchObject client.Object
	LastPatch       client.Patch
}

func (m *mockClient[T]) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	panic("implement me")
}

func (m *mockClient[T]) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if ctx == nil {
		panic("nil context")
	}
	m.GotListOpts = opts

	if m.ObjectList == nil {
		return nil
	}

	switch ref := list.(type) {
	case *corev1.PodList:
		*ref = m.ObjectList.(corev1.PodList)
	case *corev1.PersistentVolumeClaimList:
		*ref = m.ObjectList.(corev1.PersistentVolumeClaimList)
	default:
		panic(fmt.Errorf("unknown ObjectList type: %T", m.ObjectList))
	}

	return nil
}

func (m *mockClient[T]) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if ctx == nil {
		panic("nil context")
	}
	m.LastCreatedResource = obj.(T)
	m.CreateCount++
	return nil
}

func (m *mockClient[T]) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if ctx == nil {
		panic("nil context")
	}
	m.DeleteCount++
	return nil
}

func (m *mockClient[T]) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	panic("implement me")
}

func (m *mockClient[T]) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if ctx == nil {
		panic("nil context")
	}
	m.PatchCount++
	m.LastPatchObject = obj
	m.LastPatch = patch
	return nil
}

func (m *mockClient[T]) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	panic("implement me")
}

func (m *mockClient[T]) Scheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := cosmosv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	return scheme
}

type mockDiffer[T any] struct {
	StubCreates, StubUpdates, StubDeletes []T
}

func (m mockDiffer[T]) Creates() []T {
	return m.StubCreates
}

func (m mockDiffer[T]) Updates() []T {
	return m.StubUpdates
}

func (m mockDiffer[T]) Deletes() []T {
	return m.StubDeletes
}
