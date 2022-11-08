package snapshot

import (
	"strings"
	"testing"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestResourceName(t *testing.T) {
	var crd cosmosv1.HostedSnapshot
	crd.Name = "test"

	require.Equal(t, "snapshot-test", ResourceName(&crd))

	crd.Name = strings.Repeat("long", 100)
	require.LessOrEqual(t, 253, len(ResourceName(&crd)))
}
