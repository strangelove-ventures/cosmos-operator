package cosmos

import (
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestIndexOwner(t *testing.T) {
	scheme := runtime.NewScheme()
	err := cosmosv1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	t.Run("happy path", func(t *testing.T) {
		resource := &corev1.Pod{}
		crd := &cosmosv1.CosmosFullNode{}
		crd.Name = "test"

		err = ctrl.SetControllerReference(crd, resource, scheme)
		require.NoError(t, err)

		index := IndexOwner[*corev1.Pod]("CosmosFullNode")
		got := index(resource)

		require.Equal(t, []string{"test"}, got)
	})

	t.Run("no controller", func(t *testing.T) {
		index := IndexOwner[*corev1.Pod]("CosmosFullNode")
		got := index(&corev1.Pod{})

		require.Nil(t, got)
	})

	t.Run("kind mismatch", func(t *testing.T) {
		resource := &corev1.Pod{}
		crd := &cosmosv1.CosmosFullNode{}
		crd.Name = "test"

		err = ctrl.SetControllerReference(crd, resource, scheme)
		require.NoError(t, err)

		index := IndexOwner[*corev1.Pod]("SomeOtherCRD")
		got := index(&corev1.Pod{})

		require.Nil(t, got)
	})
}
