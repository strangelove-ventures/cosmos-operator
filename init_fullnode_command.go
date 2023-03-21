package main

import (
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func initFullNodeCmd() *cobra.Command {
	initFlow := &cobra.Command{
		Use:   "init-fullnode",
		Short: "Initialize a fullnode, prepping the environment for use by the chain process.",
		Long: `Initialize a fullnode including creating, moving, downloading files necessary for a chain to run.
Intended to run as an init container for a CosmosFullNode pod.

****Currently this command is a stub.****

TODO:
- Check for legacy data locations and migrate to proper PVC path. A response to https://github.com/osmosis-labs/osmosis/issues/4654.
- Manage files created from the chain's init command.
- Download genesis files.
- Download and extract snapshots.
`,
		RunE:         initFullNode,
		SilenceUsage: true,
	}

	initFlow.Flags().String("log-format", "console", "'console' or 'json'")
	initFlow.Flags().String("log-level", "info", "log level")

	if err := viper.BindPFlags(initFlow.Flags()); err != nil {
		panic(err)
	}

	return initFlow
}

func initFullNode(cmd *cobra.Command, args []string) error {
	var (
		zlog   = zapLogger(viper.GetString("log-level"), viper.GetString("log-format"))
		logger = zapr.NewLogger(zlog)
	)
	defer func() { _ = zlog.Sync() }()

	logger.Info("Command init-fullnode called, this command is currently a stub and does nothing.")
	return nil
}
