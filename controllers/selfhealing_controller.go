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
	"context"
	"errors"
	"net/http"
	"time"

	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	"github.com/strangelove-ventures/cosmos-operator/internal/cosmos"
	"github.com/strangelove-ventures/cosmos-operator/internal/fullnode"
	"github.com/strangelove-ventures/cosmos-operator/internal/healthcheck"
	"github.com/strangelove-ventures/cosmos-operator/internal/kube"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SelfHealingReconciler reconciles the self healing portion of a CosmosFullNode object
type SelfHealingReconciler struct {
	client.Client
	cacheController *cosmos.CacheController
	diskClient      *fullnode.DiskUsageCollector
	driftDetector   fullnode.DriftDetection
	pvcAutoScaler   *fullnode.PVCAutoScaler
	recorder        record.EventRecorder
}

func NewSelfHealing(
	client client.Client,
	recorder record.EventRecorder,
	statusClient *fullnode.StatusClient,
	httpClient *http.Client,
	cacheController *cosmos.CacheController,
) *SelfHealingReconciler {
	return &SelfHealingReconciler{
		Client:          client,
		cacheController: cacheController,
		diskClient:      fullnode.NewDiskUsageCollector(healthcheck.NewClient(httpClient), client),
		driftDetector:   fullnode.NewDriftDetection(cacheController),
		pvcAutoScaler:   fullnode.NewPVCAutoScaler(statusClient),
		recorder:        recorder,
	}
}

// Reconcile reconciles only the self-healing spec in CosmosFullNode. If changes needed, this controller
// updates a CosmosFullNode status subresource thus triggering another reconcile loop. The CosmosFullNode
// uses the status object to reconcile its state.
func (r *SelfHealingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName(cosmosv1.SelfHealingController)
	logger.V(1).Info("Entering reconcile loop", "request", req.NamespacedName)

	crd := new(cosmosv1.CosmosFullNode)
	if err := r.Get(ctx, req.NamespacedName, crd); err != nil {
		// Ignore not found errors because can't be fixed by an immediate requeue. We'll have to wait for next notification.
		// Also, will get "not found" error if crd is deleted.
		// No need to explicitly delete resources. Kube GC does so automatically because we set the controller reference
		// for each resource.
		return stopResult, client.IgnoreNotFound(err)
	}

	if crd.Spec.SelfHeal == nil {
		return stopResult, nil
	}

	reporter := kube.NewEventReporter(logger, r.recorder, crd)

	r.pvcAutoScale(ctx, reporter, crd)
	r.mitigateHeightDrift(ctx, reporter, crd)

	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

func (r *SelfHealingReconciler) pvcAutoScale(ctx context.Context, reporter kube.Reporter, crd *cosmosv1.CosmosFullNode) {
	if crd.Spec.SelfHeal.PVCAutoScale == nil {
		return
	}
	usage, err := r.diskClient.CollectDiskUsage(ctx, crd)
	if err != nil {
		reporter.Error(err, "Failed to collect pvc disk usage")
		// This error can be noisy so we record a generic error. Check logs for error details.
		reporter.RecordError("PVCAutoScaleCollectUsage", errors.New("failed to collect pvc disk usage"))
		return
	}
	didSignal, err := r.pvcAutoScaler.SignalPVCResize(ctx, crd, usage)
	if err != nil {
		reporter.Error(err, "Failed to signal pvc resize")
		reporter.RecordError("PVCAutoScaleSignalResize", err)
		return
	}
	if !didSignal {
		return
	}
	const msg = "PVC auto scaling requested disk expansion"
	reporter.Info(msg)
	reporter.RecordInfo("PVCAutoScale", msg)
}

func (r *SelfHealingReconciler) mitigateHeightDrift(ctx context.Context, reporter kube.Reporter, crd *cosmosv1.CosmosFullNode) {
	if crd.Spec.SelfHeal.HeightDriftMitigation == nil {
		return
	}

	const msg = "Height drift mitigation deleted pod"
	pods := r.driftDetector.LaggingPods(ctx, crd)
	if len(pods) > 0 {
		reporter.RecordInfo("HeightDriftMitigation", msg)
	}
	for _, pod := range pods {
		// CosmosFullNodeController will detect missing pod and re-create it.
		if err := r.Delete(ctx, pod); kube.IgnoreNotFound(err) != nil {
			reporter.Error(err, "Failed to delete pod", "pod", pod)
			reporter.RecordError("HeightDriftMitigationDeletePod", err)
			continue
		}
		reporter.Info(msg, "pod", pod)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SelfHealingReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager) error {
	// We do not have to index Pods because the CosmosFullNodeReconciler already does so.
	// If we repeat it here, the manager returns an error.
	return ctrl.NewControllerManagedBy(mgr).
		For(&cosmosv1.CosmosFullNode{}).
		Complete(r)
}
