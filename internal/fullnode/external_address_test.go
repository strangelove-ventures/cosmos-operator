package fullnode

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCollectExternalP2P(t *testing.T) {
	t.Parallel()

	type mockSvcClient = mockClient[*corev1.Service]
	ctx := context.Background()
	const namespace = "default"

	t.Run("happy path", func(t *testing.T) {
		stubSvcs := lo.Map(lo.Range(3), func(i, _ int) corev1.Service {
			var stubSvc corev1.Service
			stubSvc.Name = "stub" + strconv.Itoa(i)
			stubSvc.Namespace = namespace
			stubSvc.Spec.Type = corev1.ServiceTypeLoadBalancer
			stubSvc.Labels = map[string]string{kube.InstanceLabel: fmt.Sprintf("instance-%d", i)}
			stubSvc.Status = corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{IP: fmt.Sprintf("0.0.0.%d", i), Hostname: "should not see me"},
					},
				},
			}
			return stubSvc
		})

		var hostSvc corev1.Service
		hostSvc.Name = "stub-host"
		hostSvc.Namespace = namespace
		hostSvc.Spec.Type = corev1.ServiceTypeLoadBalancer
		hostSvc.Labels = map[string]string{kube.InstanceLabel: "instance-3"}
		hostSvc.Status = corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{Hostname: "host.example.com"},
				},
			},
		}

		var missing corev1.Service
		missing.Name = "stub-missing"
		missing.Namespace = namespace
		missing.Spec.Type = corev1.ServiceTypeLoadBalancer
		missing.Labels = map[string]string{kube.InstanceLabel: "instance-4"}

		var clusterIPSvc corev1.Service
		clusterIPSvc.Namespace = namespace
		clusterIPSvc.Name = "should-filter-me"
		clusterIPSvc.Labels = map[string]string{kube.InstanceLabel: "should-not-see-me"}

		var mClient mockSvcClient
		mClient.ObjectList = corev1.ServiceList{Items: append(stubSvcs, hostSvc, missing, clusterIPSvc)}

		crd := defaultCRD()
		crd.Namespace = namespace
		crd.Name = "simapp"

		got, err := CollectExternalP2P(ctx, &crd, &mClient)
		require.NoError(t, err)

		want := ExternalAddresses{
			client.ObjectKey{Name: "instance-0", Namespace: namespace}: "0.0.0.0:26656",
			client.ObjectKey{Name: "instance-1", Namespace: namespace}: "0.0.0.1:26656",
			client.ObjectKey{Name: "instance-2", Namespace: namespace}: "0.0.0.2:26656",
			client.ObjectKey{Name: "instance-3", Namespace: namespace}: "host.example.com:26656",
			client.ObjectKey{Name: "instance-4", Namespace: namespace}: "",
		}
		require.Equal(t, want, got)

		require.Len(t, mClient.GotListOpts, 3)
		var listOpt client.ListOptions
		for _, opt := range mClient.GotListOpts {
			opt.ApplyToList(&listOpt)
		}
		require.Equal(t, namespace, listOpt.Namespace)
		require.Zero(t, listOpt.Limit)
		require.Equal(t, "app.kubernetes.io/component=p2p", listOpt.LabelSelector.String())
		require.ElementsMatch(t, []string{".metadata.controller=simapp"}, strings.Split(listOpt.FieldSelector.String(), ","))
	})

	t.Run("zero state", func(t *testing.T) {
		crd := defaultCRD()
		var mClient mockSvcClient
		got, err := CollectExternalP2P(ctx, &crd, &mClient)

		require.NoError(t, err)
		require.Empty(t, got)
	})
}

func TestExternalAddresses_Incomplete(t *testing.T) {
	t.Parallel()

	addrs := ExternalAddresses{}
	// Supports scale to 0 scenario.
	require.False(t, addrs.Incomplete())

	addrs[client.ObjectKey{Name: "instance-0"}] = ""
	require.True(t, addrs.Incomplete())

	addrs[client.ObjectKey{Name: "instance-0"}] = "1.2.3.4"
	require.False(t, addrs.Incomplete())

	addrs[client.ObjectKey{Name: "instance-1"}] = ""
	require.True(t, addrs.Incomplete())

	addrs[client.ObjectKey{Name: "instance-1"}] = "host"
	require.False(t, addrs.Incomplete())
}
