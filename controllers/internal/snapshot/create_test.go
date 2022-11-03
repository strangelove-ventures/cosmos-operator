package snapshot

import (
	"context"
	"fmt"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func requireOwner(t *testing.T, crd metav1.Object, obj client.Object) {
	t.Helper()
	require.NotEmpty(t, crd.GetName())
	require.Equal(t, crd.GetName(), obj.GetOwnerReferences()[0].Name)
	require.Equal(t, "HostedSnapshot", obj.GetOwnerReferences()[0].Kind)
	require.True(t, *obj.GetOwnerReferences()[0].Controller)
}

type mockCreateClient struct {
	GotObjects []client.Object
	Err        error
}

func (m *mockCreateClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if ctx == nil {
		panic("nil context")
	}
	if len(opts) > 0 {
		panic(fmt.Errorf("expected 0 opts, got %d", len(opts)))
	}
	m.GotObjects = append(m.GotObjects, obj)
	return m.Err
}

func (m *mockCreateClient) Scheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := cosmosv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	return scheme
}

func TestCreator_Create(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		var (
			crd     cosmosv1.HostedSnapshot
			mClient mockCreateClient
			pods    = []*corev1.Pod{new(corev1.Pod), new(corev1.Pod)}
		)
		crd.Name = "create-test"

		creator := NewCreator(&mClient, func() ([]*corev1.Pod, error) {
			return pods, nil
		})

		err := creator.Create(ctx, &crd)

		require.NoError(t, err)
		require.Equal(t, 2, len(mClient.GotObjects))

		for _, pod := range mClient.GotObjects {
			requireOwner(t, &crd, pod)
		}
	})

	t.Run("builder error", func(t *testing.T) {
		t.Fatal("TODO")
	})

	t.Run("create error", func(t *testing.T) {
		t.Fatal("TODO")
	})
}
