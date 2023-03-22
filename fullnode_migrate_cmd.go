package main

import (
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func fullNodeMigrateCmd() *cobra.Command {
	migrate := &cobra.Command{
		Use:   "fullnode-migrate",
		Short: "Performs idempotent data migrations for a fullnode",
		Long: `Performs idempotent data migrations for a fullnode.
Intended to run as an init container for a CosmosFullNode pod.`,
		RunE:         migrateFullNode,
		SilenceUsage: true,
	}

	migrate.Flags().String("log-format", "console", "'console' or 'json'")
	migrate.Flags().String("log-level", "info", "log level")

	if err := viper.BindPFlags(migrate.Flags()); err != nil {
		panic(err)
	}

	return migrate
}

func migrateFullNode(cmd *cobra.Command, args []string) error {
	var (
		zlog   = zapLogger(viper.GetString("log-level"), viper.GetString("log-format"))
		logger = zapr.NewLogger(zlog)
	)
	defer func() { _ = zlog.Sync() }()

	logger.Info("Command called, this command is currently a stub and does nothing.")
	return nil
}
