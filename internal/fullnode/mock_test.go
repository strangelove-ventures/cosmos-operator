package fullnode

import (
	"context"
	"fmt"
	"sync"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockClient[T client.Object] struct {
	mu sync.Mutex

	Object       any
	GetObjectKey client.ObjectKey
	GetObjectErr error

	ObjectList  any
	GotListOpts []client.ListOption
	ListErr     error

	CreateCount      int
	LastCreateObject T
	CreatedObjects   []T

	DeleteCount int

	PatchCount      int
	LastPatchObject client.Object
	LastPatch       client.Patch

	LastUpdateObject T
	UpdateCount      int
	UpdateErr        error
}

func (m *mockClient[T]) Get(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ctx == nil {
		panic("nil context")
	}
	m.GetObjectKey = key
	if m.Object == nil {
		return m.GetObjectErr
	}

	switch ref := obj.(type) {
	case *corev1.ConfigMap:
		*ref = m.Object.(corev1.ConfigMap)
	case *corev1.PersistentVolumeClaim:
		*ref = m.Object.(corev1.PersistentVolumeClaim)
	case *cosmosv1.CosmosFullNode:
		*ref = m.Object.(cosmosv1.CosmosFullNode)
	case *snapshotv1.VolumeSnapshot:
		*ref = m.Object.(snapshotv1.VolumeSnapshot)
	default:
		panic(fmt.Errorf("unknown Object type: %T", m.ObjectList))
	}
	return m.GetObjectErr
}

func (m *mockClient[T]) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

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
	case *corev1.ServiceList:
		*ref = m.ObjectList.(corev1.ServiceList)
	case *corev1.ConfigMapList:
		*ref = m.ObjectList.(corev1.ConfigMapList)
	case *corev1.SecretList:
		*ref = m.ObjectList.(corev1.SecretList)
	case *corev1.ServiceAccountList:
		*ref = m.ObjectList.(corev1.ServiceAccountList)
	case *rbacv1.RoleList:
		*ref = m.ObjectList.(rbacv1.RoleList)
	case *rbacv1.RoleBindingList:
		*ref = m.ObjectList.(rbacv1.RoleBindingList)
	default:
		panic(fmt.Errorf("unknown ObjectList type: %T", m.ObjectList))
	}

	return m.ListErr
}

func (m *mockClient[T]) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ctx == nil {
		panic("nil context")
	}
	m.LastCreateObject = obj.(T)
	m.CreatedObjects = append(m.CreatedObjects, obj.(T))
	m.CreateCount++
	return nil
}

func (m *mockClient[T]) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ctx == nil {
		panic("nil context")
	}
	m.DeleteCount++
	return nil
}

func (m *mockClient[T]) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ctx == nil {
		panic("nil context")
	}
	m.UpdateCount++
	m.LastUpdateObject = obj.(T)
	return m.UpdateErr
}

func (m *mockClient[T]) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

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
	m.mu.Lock()
	defer m.mu.Unlock()

	scheme := runtime.NewScheme()
	if err := cosmosv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	return scheme
}

func (m *mockClient[T]) Status() client.StatusWriter {
	return m
}

func (m *mockClient[T]) RESTMapper() meta.RESTMapper {
	panic("implement me")
}
