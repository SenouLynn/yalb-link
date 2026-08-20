package stream

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Path is where the browser event stream is mounted.
const Path = "/api/events"

// KeepalivePeriod bounds how long the connection can look idle.
//
// Proxies and load balancers close connections that produce nothing, and a
// vehicle at 1 Hz on its slowest families still has quiet stretches. An SSE
// comment is the cheapest thing that keeps the socket demonstrably alive
// without inventing a telemetry event that did not happen.
const KeepalivePeriod = 15 * time.Second

// Handler serves the event stream. Each request gets its own subscription,
// bootstrapped from retained state and then followed live.
func Handler(hub *Hub, log *slog.Logger) http.HandlerFunc {
	return handler(hub, log, KeepalivePeriod)
}

// handler is Handler with the keepalive period injected, so tests can observe
// an idle stream without waiting out the production interval.
func handler(hub *Hub, log *slog.Logger, keepalivePeriod time.Duration) http.HandlerFunc {
	if log == nil {
		log = slog.Default()
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		// Tell nginx-style proxies not to buffer; a buffered event stream
		// arrives in bursts and the display stutters for no visible reason.
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		events, unsubscribe := hub.Subscribe()
		defer unsubscribe()

		conn := http.NewResponseController(w)

		// Flush the headers so the browser's EventSource opens immediately
		// rather than when the first event happens to arrive.
		if err := conn.Flush(); err != nil {
			log.DebugContext(ctx, "sse client gone before first flush", "err", err)

			return
		}

		keepalive := time.NewTicker(keepalivePeriod)
		defer keepalive.Stop()

		for {
			select {
			case <-ctx.Done():
				log.DebugContext(ctx, "sse client disconnected", "reason", "request cancelled")

				return

			case ev, ok := <-events:
				if !ok {
					log.InfoContext(ctx, "sse subscription ended",
						"reason", "dropped by hub")

					return
				}

				if err := writeEvent(w, ev); err != nil {
					log.DebugContext(ctx, "sse write failed", "err", err)

					return
				}

				if err := conn.Flush(); err != nil {
					log.DebugContext(ctx, "sse flush failed", "err", err)

					return
				}

			case <-keepalive.C:
				if _, err := w.Write([]byte(": keepalive\n\n")); err != nil {
					return
				}

				if err := conn.Flush(); err != nil {
					return
				}
			}
		}
	}
}

// writeEvent renders one event as an SSE frame.
func writeEvent(w http.ResponseWriter, ev Event) error {
	data, err := encode(ev.Message)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Name, data); err != nil {
		return fmt.Errorf("stream: writing %s event: %w", ev.Name, err)
	}

	return nil
}

// encode renders a message as single-line protobuf JSON.
//
// The compaction is load-bearing rather than tidiness: SSE terminates a data
// field at a newline, so a multi-line payload would be delivered as several
// truncated frames. protojson also perturbs its own whitespace between calls;
// compacting removes that too, which makes the wire bytes reproducible.
func encode(msg proto.Message) ([]byte, error) {
	raw, err := protojson.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("stream: encoding %T: %w", msg, err)
	}

	// protojson is compact unless Indent is configured, so the payload is one
	// SSE data line without a second JSON parsing pass.
	return raw, nil
}
