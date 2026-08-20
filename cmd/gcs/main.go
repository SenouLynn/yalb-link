// Package main assembles and runs the GCS backend.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"golang.org/x/sync/errgroup"

	"yalb.gcs/internal/bridge"
	"yalb.gcs/internal/codec"
)

// httpAddr is where the health endpoint is served.
const httpAddr = ":8080"

// shutdownGrace bounds how long the HTTP server is given to finish in-flight
// requests once the process has been told to stop.
const shutdownGrace = 5 * time.Second

// envLogLevel controls structured log verbosity.
const envLogLevel = "GCS_LOG_LEVEL"

func main() {
	log := newLogger()

	if err := run(log); err != nil {
		log.Error("gcs backend exited", "err", err)
		os.Exit(1)
	}

	log.Info("gcs backend stopped")
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	bind := codec.ResolveBind(os.LookupEnv(codec.EnvUDPBind))
	if warning := codec.PostureWarning(bind); warning != "" {
		log.Warn(warning)
	}

	group, ctx := errgroup.WithContext(ctx)

	if bind == "" {
		// An explicitly empty bind disables MAVLink while retaining health checks.
		log.Warn("MAVLink socket disabled", "reason", codec.EnvUDPBind+" set to empty")
	} else if err := startBridge(ctx, group, log, bind); err != nil {
		return err
	}

	serveHTTP(ctx, group, log)

	if err := group.Wait(); err != nil {
		return fmt.Errorf("gcs: backend stopped: %w", err)
	}

	return nil
}

// startBridge opens the MAVLink socket and runs the receive pipeline on it.
func startBridge(ctx context.Context, group *errgroup.Group, log *slog.Logger, bind string) error {
	node, err := codec.NewNode([]gomavlib.EndpointConf{
		gomavlib.EndpointUDPServer{Address: bind},
	})
	if err != nil {
		return fmt.Errorf("gcs: opening MAVLink socket on %s: %w", bind, err)
	}

	br, err := bridge.New(bridge.Config{
		Source: node,
		Sink:   bridge.LogSink{Log: log},
		Logger: log,
	})
	if err != nil {
		return fmt.Errorf("gcs: assembling bridge: %w", err)
	}

	log.Info("MAVLink bridge listening", "bind", bind,
		"heartbeat_period", codec.HeartbeatPeriod,
		"heartbeat_ttl", codec.HeartbeatTTL,
		"link_idle_timeout", codec.LinkIdleTimeout,
	)

	group.Go(func() error {
		// The bridge must stop consuming before its node closes.
		defer func() {
			if err := node.Close(); err != nil {
				log.Error("closing MAVLink node", "err", err)
			}
		}()

		return br.Run(ctx)
	})

	return nil
}

// serveHTTP runs the health server and shuts it down with the context.
func serveHTTP(ctx context.Context, group *errgroup.Group, log *slog.Logger) {
	mux := http.NewServeMux()

	// This is liveness only; it does not assert a vehicle link.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	group.Go(func() error {
		log.Info("http listening", "addr", httpAddr)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("gcs: http server on %s: %w", httpAddr, err)
		}

		return nil
	})

	group.Go(func() error {
		<-ctx.Done()

		// Shutdown gets its own bounded context after process cancellation.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownGrace)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("gcs: shutting down http server: %w", err)
		}

		return nil
	})
}

// newLogger builds the process logger.
func newLogger() *slog.Logger {
	level := slog.LevelInfo
	if strings.EqualFold(os.Getenv(envLogLevel), "debug") {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
