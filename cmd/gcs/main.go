// Package main assembles and runs the GCS backend.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
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
	"yalb.gcs/internal/command"
	"yalb.gcs/internal/mission"
	"yalb.gcs/internal/recording"
	"yalb.gcs/internal/routes"
	"yalb.gcs/internal/stream"
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

	var store *recording.Store
	if recording.ResolveEnabled(os.LookupEnv(recording.EnvEnabled)) {
		path := recording.ResolveDBPath(os.LookupEnv(recording.EnvDBPath))
		maxTotalBytes := recording.ResolveMaxTotalBytes(os.LookupEnv(recording.EnvMaxTotalBytes))
		maxTotalAge := recording.ResolveMaxTotalAge(os.LookupEnv(recording.EnvMaxTotalAge))
		var err error
		store, err = recording.Open(recording.Config{
			Path: path, Log: log, MaxTotalBytes: maxTotalBytes, MaxTotalAge: maxTotalAge,
		})
		if err != nil {
			return fmt.Errorf("gcs: opening recording store: %w", err)
		}
		defer func() {
			if err := store.Close(); err != nil {
				log.Error("closing recording store", "err", err)
			}
		}()
		log.Info("flight recording enabled", "path", path)
	} else {
		log.Info("flight recording disabled", "env", recording.EnvEnabled)
	}

	// The hub outlives any single browser and exists even without a MAVLink
	// socket, so the UI can connect and correctly show an empty fleet rather
	// than failing to load.
	hub := stream.NewHub(log, 0)
	defer hub.Close()

	var registry *command.Registry
	missionCoordinator := &mission.Coordinator{Log: log}
	if bind == "" {
		// An explicitly empty bind disables MAVLink while retaining health checks.
		log.Warn("MAVLink socket disabled", "reason", codec.EnvUDPBind+" set to empty")
	} else {
		node, err := codec.NewNode([]gomavlib.EndpointConf{gomavlib.EndpointUDPServer{Address: bind}})
		if err != nil {
			return fmt.Errorf("gcs: opening MAVLink socket on %s: %w", bind, err)
		}
		table := routes.NewTable()
		missionCoordinator.Source = node
		missionCoordinator.Routes = table
		if command.ResolveEnabled(os.LookupEnv(command.EnvEnabled)) {
			publisher := bridge.MultiSink{bridge.LogSink{Log: log}, hub}
			if store != nil {
				publisher = append(publisher, &recording.Recorder{Store: store})
			}
			registry = &command.Registry{Source: node, Routes: table, Publisher: publisher, Log: log}
			log.Info("operator commands enabled")
		} else {
			log.Info("operator commands disabled", "env", command.EnvEnabled)
		}
		if err := startBridge(ctx, group, log, bind, node, table, registry, missionCoordinator, hub, store); err != nil {
			_ = node.Close()
			return err
		}
	}

	serveHTTP(ctx, group, log, hub, store, registry, missionCoordinator)

	if err := group.Wait(); err != nil {
		return fmt.Errorf("gcs: backend stopped: %w", err)
	}

	return nil
}

// startBridge opens the MAVLink socket and runs the receive pipeline on it.
func startBridge(
	ctx context.Context,
	group *errgroup.Group,
	log *slog.Logger,
	bind string,
	node codec.FrameSource,
	table *routes.Table,
	registry *command.Registry,
	missionCoordinator *mission.Coordinator,
	hub *stream.Hub,
	store *recording.Store,
) error {
	// The route table is shared rather than left to the bridge to create,
	// because the rate requester has to send to the same links the receive loop
	// learned them from.
	sinks := bridge.MultiSink{
		bridge.LogSink{Log: log},
		&bridge.RateRequester{Source: node, Routes: table, Log: log},
	}
	if registry != nil {
		sinks = append(sinks, registry)
	}
	sinks = append(sinks, missionCoordinator)
	sinks = append(sinks, hub)
	if store != nil {
		sinks = append(sinks, &recording.Recorder{Store: store})
	}

	br, err := bridge.New(bridge.Config{
		Source: node,
		Routes: table,
		// Order matters: the discovery is logged, then acted on, then
		// published, so the log explains any request a browser sees the
		// results of.
		Sink:   sinks,
		Logger: log,
	})
	if err != nil {
		return fmt.Errorf("gcs: assembling bridge: %w", err)
	}

	log.Info("MAVLink bridge listening", "bind", bind,
		"heartbeat_period", codec.HeartbeatPeriod,
		"heartbeat_ttl", codec.HeartbeatTTL,
		"link_idle_timeout", codec.LinkIdleTimeout,
		"requested_families", len(bridge.DefaultRates),
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

// serveHTTP runs the health and event servers and shuts them down with the
// context.
func serveHTTP(ctx context.Context, group *errgroup.Group, log *slog.Logger, hub *stream.Hub, store *recording.Store, registry *command.Registry, missionCoordinator *mission.Coordinator) {
	mux := newHTTPMux(log, hub, store, registry, missionCoordinator)

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		// Event-stream handlers block until their request context is cancelled.
		// Shutdown waits for active requests but does not cancel them, so
		// without this an open browser tab would hold the process past its
		// shutdown grace and turn a clean stop into a timeout error. Deriving
		// request contexts from the process context makes cancellation reach
		// the handlers. No WriteTimeout is set for the same reason: a live
		// stream is a long-lived response, not a stalled one.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	group.Go(func() error {
		log.Info("http listening", "addr", httpAddr, "events", stream.Path)

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

func newHTTPMux(log *slog.Logger, hub *stream.Hub, store *recording.Store, registry *command.Registry, missionCoordinator *mission.Coordinator) *http.ServeMux {
	mux := http.NewServeMux()

	// This is liveness only; it does not assert a vehicle link.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET "+stream.Path, stream.Handler(hub, log))
	mux.HandleFunc(mission.DownloadPattern, mission.DownloadHandler(missionCoordinator))
	if registry != nil {
		mux.HandleFunc(command.ArmPattern, command.ArmHandler(registry))
		mux.HandleFunc(command.ResolvePattern, command.ResolveHandler(registry))
	}
	if store != nil {
		mux.HandleFunc("POST /api/recordings/start", recording.StartHandler(store))
		mux.HandleFunc("POST /api/recordings/stop", recording.StopHandler(store))
		mux.HandleFunc("GET /api/recordings", recording.ListHandler(store))
		mux.HandleFunc(recording.DeletePattern, recording.DeleteHandler(store))
		mux.HandleFunc(recording.EventsPattern, recording.ReplayEventsHandler(store))
	}

	return mux
}

// newLogger builds the process logger.
func newLogger() *slog.Logger {
	level := slog.LevelInfo
	if strings.EqualFold(os.Getenv(envLogLevel), "debug") {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
