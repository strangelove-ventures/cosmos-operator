/*
Copyright 2024 B-Harvest Corporation.
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

package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	opcmd "github.com/bharvest-devops/cosmos-operator/cmd"
	"github.com/bharvest-devops/cosmos-operator/controllers"
	"github.com/bharvest-devops/cosmos-operator/internal/cosmos"
	"github.com/bharvest-devops/cosmos-operator/internal/fullnode"
	"github.com/bharvest-devops/cosmos-operator/internal/version"
	"github.com/go-logr/zapr"
	"github.com/pkg/profile"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	// Add Pprof endpoints.
	_ "net/http/pprof"

	cosmosv1 "github.com/bharvest-devops/cosmos-operator/api/v1"
	cosmosv1alpha1 "github.com/bharvest-devops/cosmos-operator/api/v1alpha1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(snapshotv1.AddToScheme(scheme))

	utilruntime.Must(cosmosv1.AddToScheme(scheme))
	utilruntime.Must(cosmosv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	root := rootCmd()

	ctx := ctrl.SetupSignalHandler()

	if err := root.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

// root command flags
var (
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
	profileMode          string
	logLevel             string
	logFormat            string
)

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Short:        "Run the operator",
		Use:          "manager",
		Version:      version.AppVersion(),
		RunE:         startManager,
		SilenceUsage: true,
	}

	root.Flags().StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	root.Flags().StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	root.Flags().BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	root.Flags().StringVar(&profileMode, "profile", "", "Enable profiling and save profile to working dir. (Must be one of 'cpu', or 'mem'.)")
	root.Flags().StringVar(&logLevel, "log-level", "info", "Logging level one of 'error', 'info', 'debug'")
	root.Flags().StringVar(&logFormat, "log-format", "console", "Logging format one of 'console' or 'json'")

	if err := viper.BindPFlags(root.Flags()); err != nil {
		panic(err)
	}

	// Add subcommands here
	root.AddCommand(opcmd.HealthCheckCmd())
	root.AddCommand(opcmd.VersionCheckCmd(scheme))
	root.AddCommand(&cobra.Command{
		Short: "Print the version",
		Use:   "version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("App Version:", version.AppVersion())
			fmt.Println("Docker Tag:", version.DockerTag())
		},
	})

	return root
}

func startManager(cmd *cobra.Command, args []string) error {
	go func() {
		setupLog.Info("Serving pprof endpoints at localhost:6060/debug/pprof")
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			setupLog.Error(err, "Pprof server exited with error")
		}
	}()

	logger := opcmd.ZapLogger(logLevel, logFormat)
	defer func() { _ = logger.Sync() }()
	ctrl.SetLogger(zapr.NewLogger(logger))

	if profileMode != "" {
		defer profile.Start(profileOpts(profileMode)...).Stop()
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "16e1bc09.bharvest",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	ctx := cmd.Context()

	// CacheController which fetches CometBFT status in the background.
	httpClient := &http.Client{Timeout: 30 * time.Second}
	statusClient := fullnode.NewStatusClient(mgr.GetClient())
	cometClient := cosmos.NewCometClient(httpClient)
	cacheController := cosmos.NewCacheController(
		cosmos.NewStatusCollector(cometClient, 5*time.Second),
		mgr.GetClient(),
		mgr.GetEventRecorderFor(cosmos.CacheControllerName),
	)
	defer func() { _ = cacheController.Close() }()
	if err = cacheController.SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create CosmosCache controller: %w", err)
	}

	// The primary controller for CosmosFullNode.
	if err = controllers.NewFullNode(
		mgr.GetClient(),
		mgr.GetEventRecorderFor(cosmosv1.CosmosFullNodeController),
		statusClient,
		cacheController,
	).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create CosmosFullNode controller: %w", err)
	}

	// An ancillary controller that supports CosmosFullNode.
	if err = controllers.NewSelfHealing(
		mgr.GetClient(),
		mgr.GetEventRecorderFor(cosmosv1.SelfHealingController),
		statusClient,
		httpClient,
		cacheController,
	).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create SelfHealing controller: %w", err)
	}

	// Test for presence of VolumeSnapshot CRD.
	snapshotErr := controllers.IndexVolumeSnapshots(ctx, mgr)
	if snapshotErr != nil {
		setupLog.Info("Warning: VolumeSnapshot CRD not found, StatefulJob and ScheduledVolumeSnapshot controllers will be disabled")
	}

	// StatefulJobs
	jobCtl := controllers.NewStatefulJob(
		mgr.GetClient(),
		mgr.GetEventRecorderFor(cosmosv1alpha1.StatefulJobController),
		snapshotErr != nil,
	)

	if err = jobCtl.SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create StatefulJob controller: %w", err)
	}

	// ScheduledVolumeSnapshots
	if err = controllers.NewScheduledVolumeSnapshotReconciler(
		mgr.GetClient(),
		mgr.GetEventRecorderFor(cosmosv1alpha1.ScheduledVolumeSnapshotController),
		statusClient,
		cacheController,
		snapshotErr != nil,
	).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create ScheduledVolumeSnapshot controller: %w", err)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	setupLog.Info("Starting Cosmos Operator manager", "version", version.AppVersion())
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}

	return nil
}

func profileOpts(mode string) []func(*profile.Profile) {
	opts := []func(*profile.Profile){profile.ProfilePath("."), profile.NoShutdownHook}
	switch mode {
	case "cpu":
		return append(opts, profile.CPUProfile)
	case "mem":
		return append(opts, profile.MemProfile)
	default:
		panic(fmt.Errorf("unknown profile mode %q", mode))
	}
}
