package statefuljob

import (
	"strings"
	"testing"

	cosmosalpha "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestResourceName(t *testing.T) {
	var crd cosmosalpha.StatefulJob
	crd.Name = "test"

	require.Equal(t, "stateful-job-test", ResourceName(&crd))

	crd.Name = strings.Repeat("long", 100)
	name := ResourceName(&crd)
	require.LessOrEqual(t, 253, len(name))
	require.Contains(t, name, "stateful-job-")
}
