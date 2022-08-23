package fullnode

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/samber/lo"
	"github.com/strangelove-ventures/cosmos-operator/controllers/internal/kube"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCollectP2PAddresses(t *testing.T) {
	type (
		mockSvcClient = mockClient[*corev1.Service]
	)
	stubSvcs := lo.Map(lo.Range(3), func(i, _ int) corev1.Service {
		var stubSvc corev1.Service
		stubSvc.Name = "stub" + strconv.Itoa(i)
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
	missing.Labels = map[string]string{kube.InstanceLabel: "instance-4"}

	var mClient mockSvcClient
	mClient.ObjectList = corev1.ServiceList{Items: append(stubSvcs, hostSvc, missing)}

	crd := defaultCRD()
	crd.Namespace = "addresses"
	crd.Name = "simapp"

	got, err := CollectP2PAddresses(context.Background(), &crd, &mClient)
	require.NoError(t, err)

	want := ExternalAddresses{
		"instance-0": "0.0.0.0",
		"instance-1": "0.0.0.1",
		"instance-2": "0.0.0.2",
		"instance-3": "host.example.com",
		"instance-4": "",
	}
	require.Equal(t, want, got)

	require.Len(t, mClient.GotListOpts, 3)
	var listOpt client.ListOptions
	for _, opt := range mClient.GotListOpts {
		opt.ApplyToList(&listOpt)
	}
	require.Equal(t, "addresses", listOpt.Namespace)
	require.Zero(t, listOpt.Limit)
	require.Equal(t, "app.kubernetes.io/component=p2p,app.kubernetes.io/name=simapp-mainnet-fullnode", listOpt.LabelSelector.String())
	require.Equal(t, ".metadata.controller=simapp", listOpt.FieldSelector.String())
}

func TestExternalAddresses_Incomplete(t *testing.T) {
	addrs := ExternalAddresses{}
	// Supports scale to 0 scenario.
	require.False(t, addrs.Incomplete())

	addrs["instance-0"] = ""
	require.True(t, addrs.Incomplete())

	addrs["instance-0"] = "1.2.3.4"
	require.False(t, addrs.Incomplete())

	addrs["instance-1"] = ""
	require.True(t, addrs.Incomplete())

	addrs["instance-1"] = "host"
	require.False(t, addrs.Incomplete())
}
