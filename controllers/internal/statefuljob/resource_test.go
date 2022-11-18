package statefuljob

import (
	"strings"
	"testing"

	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestResourceName(t *testing.T) {
	t.Parallel()

	var crd cosmosalpha.StatefulJob
	crd.Name = "test"

	require.Equal(t, "test", ResourceName(&crd))

	crd.Name = strings.Repeat("long", 100)
	name := ResourceName(&crd)
	require.LessOrEqual(t, 253, len(name))
}
