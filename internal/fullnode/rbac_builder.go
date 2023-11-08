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
	return crd.Name + "-vc-sa"
}

func roleName(crd *cosmosv1.CosmosFullNode) string {
	return crd.Name + "-vc-r"
}

func roleBindingName(crd *cosmosv1.CosmosFullNode) string {
	return crd.Name + "-vc-rb"
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

// BuildRoles returns a list of role bindings given the crd.
//
// Creates a single role binding for the version check.
func BuildRoles(crd *cosmosv1.CosmosFullNode) []diff.Resource[*rbacv1.Role] {
	diffCr := make([]diff.Resource[*rbacv1.Role], 1)
	cr := rbacv1.Role{
		TypeMeta: v1.TypeMeta{
			Kind:       "Role",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      roleName(crd),
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
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"cosmos.strange.love"},
				Resources: []string{"cosmosfullnodes/status"},
				Verbs:     []string{"patch"},
			},
		},
	}

	cr.Labels = defaultLabels(crd, kube.ComponentLabel, "vc")

	diffCr[0] = diff.Adapt(&cr, 0)

	return diffCr
}

// BuildRoles returns a list of role binding bindings given the crd.
//
// Creates a single role binding binding for the version check.
func BuildRoleBindings(crd *cosmosv1.CosmosFullNode) []diff.Resource[*rbacv1.RoleBinding] {
	diffCrb := make([]diff.Resource[*rbacv1.RoleBinding], 1)
	crb := rbacv1.RoleBinding{
		TypeMeta: v1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      roleBindingName(crd),
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
			Kind:     "Role",
			Name:     roleName(crd),
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	crb.Labels = defaultLabels(crd, kube.ComponentLabel, "vc")

	diffCrb[0] = diff.Adapt(&crb, 0)

	return diffCrb
}
