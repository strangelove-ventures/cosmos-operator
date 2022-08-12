package fullnode

import (
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RequireValidMetadata asserts valid metadata properties such as name and label length.
func RequireValidMetadata(t *testing.T, obj client.Object) {
	t.Helper()

	require.LessOrEqual(t, len(obj.GetName()), 253)
	for k, v := range obj.GetLabels() {
		require.LessOrEqual(t, len(v), 63, k)
	}
	for k, v := range obj.GetAnnotations() {
		require.LessOrEqual(t, len(v), 63, k)
	}
}
