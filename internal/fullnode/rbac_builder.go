package fullnode

import (
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/diff"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func serviceAccountName(crd *cosmosv1.CosmosFullNode) string {
	return crd.Name + "-vc"
}

func clusterRoleName(crd *cosmosv1.CosmosFullNode) string {
	return crd.Name + "-cr"
}

// BuildServiceAccounts returns a list of service accounts given the crd.
//
// Creates a single service account for the version check.
func BuildServiceAccounts(crd *cosmosv1.CosmosFullNode) []diff.Resource[*corev1.ServiceAccount] {
	diffSa := make([]diff.Resource[*corev1.ServiceAccount], 1)
	sa := corev1.ServiceAccount{
		TypeMeta: v1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      serviceAccountName(crd),
			Namespace: crd.Namespace,
		},
	}

	sa.Labels = defaultLabels(crd, kube.ComponentLabel, "vc")

	diffSa[0] = diff.Adapt(&sa, 0)

	return diffSa
}

// BuildClusterRoles returns a list of cluster roles given the crd.
//
// Creates a single cluster role for the version check.
func BuildClusterRoles(crd *cosmosv1.CosmosFullNode) []diff.Resource[*rbacv1.ClusterRole] {
	diffCr := make([]diff.Resource[*rbacv1.ClusterRole], 1)
	cr := rbacv1.ClusterRole{
		TypeMeta: v1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      clusterRoleName(crd),
			Namespace: crd.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""}, // core API group
				Resources: []string{"namespaces", "pods"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{"cosmos.strange.love"},
				Resources: []string{"cosmosfullnodes"},
				Verbs:     []string{"get", "list", "patch"},
			},
		},
	}

	cr.Labels = defaultLabels(crd, kube.ComponentLabel, "vc")

	diffCr[0] = diff.Adapt(&cr, 0)

	return diffCr
}

// BuildClusterRoles returns a list of cluster role bindings given the crd.
//
// Creates a single cluster role binding for the version check.
func BuildClusterRoleBindings(crd *cosmosv1.CosmosFullNode) []diff.Resource[*rbacv1.ClusterRoleBinding] {
	diffCrb := make([]diff.Resource[*rbacv1.ClusterRoleBinding], 1)
	crb := rbacv1.ClusterRoleBinding{
		TypeMeta: v1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      crd.Name + "-crb",
			Namespace: crd.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName(crd),
				Namespace: crd.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     clusterRoleName(crd),
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	crb.Labels = defaultLabels(crd, kube.ComponentLabel, "vc")

	diffCrb[0] = diff.Adapt(&crb, 0)

	return diffCrb
}
