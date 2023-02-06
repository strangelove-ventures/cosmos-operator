package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/go-logr/zapr"
	"github.com/pkg/profile"
	"github.com/strangelove-ventures/cosmos-operator/controllers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

func startManager(ctx context.Context) error {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
		profileMode          string
		logLevel             string
		logFormat            string
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&profileMode, "profile", "", "Enable profiling and save profile to working dir. (Must be one of 'cpu', or 'mem'.)")
	flag.StringVar(&logLevel, "log-level", "info", "Logging level one of 'error', 'info', 'debug'")
	flag.StringVar(&logFormat, "log-format", "console", "Logging format one of 'console' or 'json'")
	flag.Parse()

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

	if err = controllers.NewFullNode(
		mgr.GetClient(),
		mgr.GetEventRecorderFor("CosmosFullNode"),
	).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create CosmosFullNode controller: %w", err)
	}

	if err = controllers.NewStatefulJob(
		mgr.GetClient(),
		mgr.GetEventRecorderFor("StatefulJob"),
	).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create StatefulJob controller: %w", err)
	}

	if err = controllers.NewScheduledVolumeSnapshotReconciler(
		mgr.GetClient(),
		mgr.GetEventRecorderFor("ScheduledVolumeSnapshot"),
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
