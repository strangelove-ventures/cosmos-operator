package main

import (
	"context"
	"net/http"
	"path"
	"time"

	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/strangelove-ventures/cosmos-operator/internal/cosmos"
	"github.com/strangelove-ventures/cosmos-operator/internal/fullnode"
	"github.com/strangelove-ventures/cosmos-operator/internal/healthcheck"
	"golang.org/x/sync/errgroup"
)

func healthcheckCmd() *cobra.Command {
	hc := &cobra.Command{
		Short:        "Start health check probe",
		Use:          "healthcheck",
		RunE:         startHealthCheckServer,
		SilenceUsage: true,
	}

	hc.Flags().String("rpc-host", "http://localhost:26657", "tendermint rpc endpoint")
	hc.Flags().String("log-format", "console", "'console' or 'json'")
	hc.Flags().Duration("timeout", 5*time.Second, "how long to wait before timing out requests to rpc-host")
	hc.Flags().String("addr", ":1251", "listen address for server to bind")

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

		httpClient = &http.Client{Timeout: 30 * time.Second}
		tmClient   = cosmos.NewTendermintClient(httpClient)

		zlog   = zapLogger("info", viper.GetString("log-format"))
		logger = zapr.NewLogger(zlog)
	)
	defer func() { _ = zlog.Sync() }()

	var (
		tm   = healthcheck.NewTendermint(logger, tmClient, rpcHost, timeout)
		disk = healthcheck.DiskUsage(fullnode.ChainHomeDir)
	)
	router := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch path.Clean(r.URL.Path) {
		case "/disk":
			disk.ServeHTTP(w, r)
		default:
			tm.ServeHTTP(w, r)
		}
	})

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      router,
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
