package connection

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"github.com/bluenviron/gomavlib/v3/pkg/dialect"
	"github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
	"github.com/bluenviron/gomavlib/v3/pkg/frame"
	"github.com/bluenviron/gomavlib/v3/pkg/streamwriter"
	"yalb.gcs/internal/bridge"
	"yalb.gcs/internal/codec"
	"yalb.gcs/internal/routes"
)

func TestSharedBridgeRoutesSurviveConnectionHandoff(t *testing.T) {
	devices := []Device{{ID: "a", Kind: "serial", Path: "a"}, {ID: "b", Kind: "serial", Path: "b"}}
	writers := map[string]*streamwriter.Writer{}
	peers := []net.Conn{}
	m := New(Config{Inventory: inventoryFunc(func(context.Context) ([]Device, error) { return devices, nil }), Open: func(d Device, _ Settings, label string) (codec.FrameSource, error) {
		local, peer := net.Pipe()
		peers = append(peers, peer)
		first := true
		node, err := codec.NewNode([]gomavlib.EndpointConf{gomavlib.EndpointCustomClient{Label: label, Connect: func(ctx context.Context) (net.Conn, error) {
			if first {
				first = false
				return local, nil
			}
			<-ctx.Done()
			return nil, ctx.Err()
		}}})
		if err != nil {
			return nil, err
		}
		drw := &dialect.ReadWriter{Dialect: ardupilotmega.Dialect}
		if err := drw.Initialize(); err != nil {
			t.Fatal(err)
		}
		fw := &frame.Writer{ByteWriter: peer, DialectRW: drw}
		if err := fw.Initialize(); err != nil {
			t.Fatal(err)
		}
		sw := &streamwriter.Writer{FrameWriter: fw, Version: streamwriter.V2, SystemID: 1, ComponentID: 1}
		if err := sw.Initialize(); err != nil {
			t.Fatal(err)
		}
		writers[d.ID] = sw
		go func() { _, _ = io.Copy(io.Discard, peer) }()
		return &serialSource{Node: node, conn: &serialConn{ReadWriteCloser: local}}, nil
	}})
	defer func() {
		for _, p := range peers {
			_ = p.Close()
		}
	}()
	defer m.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	table := routes.NewTable()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	b, err := bridge.New(bridge.Config{Source: m, Routes: table, Logger: log, Sink: &bridge.RateRequester{Source: m, Routes: table, Log: log}})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- b.Run(ctx) }()
	send := func(id string, sysid byte) {
		t.Helper()
		writers[id].SystemID = sysid
		if err := writers[id].Write(&ardupilotmega.MessageHeartbeat{Type: 2, Autopilot: 3, MavlinkVersion: 3}); err != nil {
			t.Fatal(err)
		}
	}
	await := func(check func() bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			select {
			case err := <-done:
				t.Fatalf("bridge stopped: %v", err)
			default:
			}
			if check() {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("route state deadline")
	}
	key := routes.Key{SysID: 1, CompID: 1}
	route := func() codec.LinkID { r, _ := table.Lookup(key, time.Now().UnixMilli()); return r.Link }
	if _, err := m.Connect(ctx, "a", Settings{57600}); err != nil {
		t.Fatal(err)
	}
	send("a", 1)
	await(func() bool { return route() != "" })
	aLink := route()
	if _, err := m.Connect(ctx, "b", Settings{57600}); err != nil {
		t.Fatal(err)
	}
	send("b", 2)
	send("b", 1)
	await(func() bool { return route() != "" && route() != aLink })
	bLink := route()
	statuses := m.Connections()
	if len(statuses[0].VehicleKeys) != 0 || len(statuses[1].VehicleKeys) != 2 {
		t.Fatalf("attribution: %+v", statuses)
	}
	if err := m.Disconnect(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if route() != bLink {
		t.Fatal("closing old source removed the new route")
	}
	if err := m.Disconnect(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	await(func() bool { return table.Len() == 0 })
	if _, err := m.Connect(ctx, "a", Settings{57600}); err != nil {
		t.Fatal(err)
	}
	send("a", 1)
	await(func() bool { return route() != "" && route() != aLink })
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
