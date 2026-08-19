// Package main is the entry point for the gcs backend server.
//
// It owns exactly three things: reading the environment, assembling the
// bridge, and coordinating shutdown. Every decision about what a frame means
// belongs below this file — the pipeline is codec -> fold -> sink, and none of
// those layers knows this file exists.
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

// httpAddr is where the health endpoint and, from Tier 6, the Connect services
// are served.
const httpAddr = ":8080"

// shutdownGrace bounds how long the HTTP server is given to finish in-flight
// requests once the process has been told to stop.
const shutdownGrace = 5 * time.Second

// envLogLevel switches the logger to debug, which is where per-frame telemetry
// lines live. Default is info: three SITL instances produce a few hundred
// telemetry events a second, and at debug the fleet events they contextualise
// are unreadable.
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
	// NotifyContext, not a bare signal channel: it makes cancellation the one
	// shutdown mechanism, so the HTTP server and the bridge stop for the same
	// reason and in a defined order rather than racing a global.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	bind := codec.ResolveBind(os.LookupEnv(codec.EnvUDPBind))
	if warning := codec.PostureWarning(bind, os.Getenv(codec.EnvSigningKey)); warning != "" {
		log.Warn(warning)
	}

	group, ctx := errgroup.WithContext(ctx)

	if bind == "" {
		// Explicitly empty is "no socket", not "default socket" — see
		// codec.ResolveBind. Serving health without a bridge is a valid
		// configuration for a container being smoke-tested, and it is worth a
		// line so nobody debugs a silent fleet for ten minutes.
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
	// The Tier 5 plan put a hand-written internal/transport package here,
	// wrapping a net.PacketConn behind typed channels. There is no seam for
	// it: gomavlib owns its socket through an EndpointConf and takes an
	// endpoint, not a byte stream, so a PacketConn underneath it would mean
	// reimplementing framing. codec.FrameSource is the transport port that
	// plan was reaching for, and it already exists.
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
		// Close on the way out rather than deferring in run(): the node owns
		// goroutines, and closing it before Run has returned would tear the
		// event channel out from under the loop reading it.
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

	// Still a liveness probe, not a readiness one: it reports that the process
	// is up, and says nothing about Redis or the SITL link. Tier 5 Chapter 6
	// replaces it with the real thing once there is a Redis client to ping and
	// a link state to report; until then a green /healthz that claimed to
	// check those would be worse than one that visibly does not.
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

		// ErrServerClosed is what a successful Shutdown looks like from here.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("gcs: http server on %s: %w", httpAddr, err)
		}

		return nil
	})

	group.Go(func() error {
		<-ctx.Done()

		// A fresh context: the one that just fired is already cancelled, and
		// passing it would make Shutdown abandon in-flight requests instantly
		// rather than giving them the grace period.
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
