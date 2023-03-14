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

package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/go-logr/zapr"
	"github.com/pkg/profile"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/strangelove-ventures/cosmos-operator/controllers"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	// Add Pprof endpoints.
	_ "net/http/pprof"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	cosmosv1alpha1 "github.com/strangelove-ventures/cosmos-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
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

var vcsRevision = func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return ""
}()

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
		Version:      vcsRevision,
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
	root.AddCommand(healthcheckCmd())

	return root
}

func startManager(cmd *cobra.Command, args []string) error {
	go func() {
		setupLog.Info("Serving pprof endpoints at localhost:6060/debug/pprof")
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			setupLog.Error(err, "Pprof server exited with error")
		}
	}()

	logger := zapLogger(logLevel, logFormat)
	defer func() { _ = logger.Sync() }()
	ctrl.SetLogger(zapr.NewLogger(logger))

	if profileMode != "" {
		defer profile.Start(profileOpts(profileMode)...).Stop()
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "16e1bc09.strange.love",
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

	if err = controllers.NewFullNode(
		mgr.GetClient(),
		mgr.GetEventRecorderFor(cosmosv1.CosmosFullNodeController),
	).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create CosmosFullNode controller: %w", err)
	}

	if err = controllers.NewSelfHealing(
		mgr.GetClient(),
		mgr.GetEventRecorderFor(cosmosv1.SelfHealingController),
	).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create SelfHealing controller: %w", err)
	}

	if err = controllers.NewStatefulJob(
		mgr.GetClient(),
		mgr.GetEventRecorderFor(cosmosv1alpha1.StatefulJobController),
	).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create StatefulJob controller: %w", err)
	}

	if err = controllers.NewScheduledVolumeSnapshotReconciler(
		mgr.GetClient(),
		mgr.GetEventRecorderFor(cosmosv1alpha1.ScheduledVolumeSnapshotController),
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

	setupLog.Info("starting manager")
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
