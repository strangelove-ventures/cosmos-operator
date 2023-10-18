package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/strangelove-ventures/cosmos-operator/internal/cosmos"
	"github.com/strangelove-ventures/cosmos-operator/internal/healthcheck"
	"golang.org/x/sync/errgroup"
)

func HealthCheckCmd() *cobra.Command {
	hc := &cobra.Command{
		Short:        "Start health check probe",
		Use:          "healthcheck",
		RunE:         startHealthCheckServer,
		SilenceUsage: true,
	}

	hc.Flags().String("rpc-host", "http://localhost:26657", "CometBFT rpc endpoint")
	hc.Flags().String("log-format", "console", "'console' or 'json'")
	hc.Flags().Duration("timeout", 5*time.Second, "how long to wait before timing out requests to rpc-host")
	hc.Flags().String("addr", fmt.Sprintf(":%d", healthcheck.Port), "listen address for server to bind")

	if err := viper.BindPFlags(hc.Flags()); err != nil {
		panic(err)
	}

	return hc
}

func startHealthCheckServer(cmd *cobra.Command, args []string) error {
	var (
		listenAddr = viper.GetString("addr")
		rpcHost    = viper.GetString("rpc-host")
		timeout    = viper.GetDuration("timeout")

		httpClient  = &http.Client{Timeout: 30 * time.Second}
		cometClient = cosmos.NewCometClient(httpClient)

		zlog   = ZapLogger("info", viper.GetString("log-format"))
		logger = zapr.NewLogger(zlog)
	)
	defer func() { _ = zlog.Sync() }()

	mux := http.NewServeMux()
	mux.Handle("/", healthcheck.NewComet(logger, cometClient, rpcHost, timeout))
	mux.HandleFunc("/disk", healthcheck.DiskUsage)

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	var eg errgroup.Group
	eg.Go(func() error {
		logger.Info("Healthcheck server listening", "addr", listenAddr, "rpcHost", rpcHost)
		return srv.ListenAndServe()
	})
	eg.Go(func() error {
		<-cmd.Context().Done()
		logger.Info("Healthcheck server shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	})

	return eg.Wait()
}
