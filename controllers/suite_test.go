/*
Copyright 2022 Strangelove Ventures LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	cosmosv1alpha1 "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

func TestAPIs(t *testing.T) {
	t.Skip("TODO: Implement test. Always skipping because of dependency issues.")
	// unable to start control plane itself: failed to start the controlplane. retried 5 times: fork/exec /usr/local/kubebuilder/bin/etcd: no such file or directory

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	t.Cleanup(func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("failed to stop test environment: %v", err)
		}
	})

	cfg, err := testEnv.Start()

	require.NoError(t, err)
	require.NotNil(t, cfg)

	err = cosmosv1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	err = cosmosv1alpha1.AddToScheme(scheme.Scheme)

	//+kubebuilder:scaffold:scheme

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)
	require.NotNil(t, k8sClient)
}
