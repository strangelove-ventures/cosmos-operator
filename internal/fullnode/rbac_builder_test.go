package fullnode

import (
	"testing"

	"github.com/stretchr/testify/require"

	rbacv1 "k8s.io/api/rbac/v1"
)

func TestBuildRBAC(t *testing.T) {
	t.Parallel()

	t.Run("build rbac", func(t *testing.T) {
		crd := defaultCRD()
		crd.Spec.Replicas = 3
		crd.Name = "hub"
		crd.Namespace = "test"
		crd.Spec.ChainSpec.Network = "testnet"
		crd.Spec.PodTemplate.Image = "gaia:v6.0.0"

		sas := BuildServiceAccounts(&crd)

		require.Len(t, sas, 1) // 1 svc account in the namespace

		sa := sas[0].Object()

		require.Equal(t, "hub-vc-sa", sa.Name)
		require.Equal(t, "test", sa.Namespace)

		wantLabels := map[string]string{
			"app.kubernetes.io/created-by": "cosmos-operator",
			"app.kubernetes.io/name":       "hub",
			"app.kubernetes.io/component":  "vc",
			"app.kubernetes.io/version":    "v6.0.0",
			"cosmos.strange.love/network":  "testnet",
			"cosmos.strange.love/type":     "FullNode",
		}
		require.Equal(t, wantLabels, sa.Labels)

		roles := BuildRoles(&crd)

		require.Len(t, roles, 1) // 1 role in the namespace

		role := roles[0].Object()

		require.Equal(t, "hub-vc-r", role.Name)
		require.Equal(t, "test", role.Namespace)

		wantLabels = map[string]string{
			"app.kubernetes.io/created-by": "cosmos-operator",
			"app.kubernetes.io/name":       "hub",
			"app.kubernetes.io/component":  "vc",
			"app.kubernetes.io/version":    "v6.0.0",
			"cosmos.strange.love/network":  "testnet",
			"cosmos.strange.love/type":     "FullNode",
		}
		require.Equal(t, wantLabels, role.Labels)

		require.Equal(t, []rbacv1.PolicyRule{
			{
				APIGroups: []string{""}, // core API group
				Resources: []string{"namespaces", "pods"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{"cosmos.strange.love"},
				Resources: []string{"cosmosfullnodes"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"cosmos.strange.love"},
				Resources: []string{"cosmosfullnodes/status"},
				Verbs:     []string{"patch"},
			},
		}, role.Rules)

		rbs := BuildRoleBindings(&crd)

		require.Len(t, rbs, 1) // 1 role in the namespace

		rb := rbs[0].Object()

		require.Equal(t, "hub-vc-rb", rb.Name)
		require.Equal(t, "test", rb.Namespace)

		wantLabels = map[string]string{
			"app.kubernetes.io/created-by": "cosmos-operator",
			"app.kubernetes.io/name":       "hub",
			"app.kubernetes.io/component":  "vc",
			"app.kubernetes.io/version":    "v6.0.0",
			"cosmos.strange.love/network":  "testnet",
			"cosmos.strange.love/type":     "FullNode",
		}
		require.Equal(t, wantLabels, rb.Labels)

		require.Len(t, rb.Subjects, 1)
		require.Equal(t, rb.Subjects[0].Name, "hub-vc-sa")

		require.Equal(t, rb.RoleRef.Name, "hub-vc-r")
	})
}
