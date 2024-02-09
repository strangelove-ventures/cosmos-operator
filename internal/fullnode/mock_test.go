package fullnode

import (
	"context"
	"fmt"
	v1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sync"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
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

	mapper meta.RESTMapper
}

func (m *mockClient[T]) SubResource(subResource string) client.SubResourceClient {
	return &mockSubResourceClient[T]{client: m, subResource: subResource}
}

func (m *mockClient[T]) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (m *mockClient[T]) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return false, nil
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
	return m.SubResource("status")
}

func (m *mockClient[T]) RESTMapper() meta.RESTMapper {
	return m.mapper
}

type mockSubResourceClient[T client.Object] struct {
	client      *mockClient[T]
	subResource string
}

func (m *mockSubResourceClient[T]) Get(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceGetOption) error {
	panic("mockSubResourceClient does not support get")
}

func (m *mockSubResourceClient[T]) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	switch m.subResource {
	case "eviction":
		_, isEviction := subResource.(*v1.Eviction)
		if !isEviction {
			_, isEviction = subResource.(*v1.Eviction)
		}
		if !isEviction {
			return apierrors.NewBadRequest(fmt.Sprintf("got invalid type %t, expected Eviction", subResource))
		}
		if _, isPod := obj.(*corev1.Pod); !isPod {
			return apierrors.NewNotFound(schema.GroupResource{}, "")
		}

		return m.client.Delete(ctx, obj)
	default:
		return fmt.Errorf("fakeSubResourceWriter does not support create for %s", m.subResource)
	}
}

func (m *mockSubResourceClient[T]) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	updateOptions := client.SubResourceUpdateOptions{}
	updateOptions.ApplyOptions(opts)

	body := obj
	if updateOptions.SubResourceBody != nil {
		body = updateOptions.SubResourceBody
	}

	return m.client.Update(ctx, body, &updateOptions.UpdateOptions)
}

func (m *mockSubResourceClient[T]) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	patchOptions := client.SubResourcePatchOptions{}
	patchOptions.ApplyOptions(opts)

	body := obj
	if patchOptions.SubResourceBody != nil {
		body = patchOptions.SubResourceBody
	}

	// this is necessary to identify that last call was made for status patch, through stack trace.
	if m.subResource == "status" {
		return m.statusPatch(ctx, body, patch, patchOptions)
	}

	return m.client.Patch(ctx, body, patch, &patchOptions.PatchOptions)
}

func (sw *mockSubResourceClient[T]) statusPatch(ctx context.Context, body client.Object, patch client.Patch, patchOptions client.SubResourcePatchOptions) error {
	return sw.client.Patch(ctx, body, patch, &patchOptions.PatchOptions)
}
