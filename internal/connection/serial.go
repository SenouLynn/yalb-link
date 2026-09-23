package connection

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/bluenviron/gomavlib/v3"
	"go.bug.st/serial"
	"yalb.gcs/internal/codec"
)

// OpenSerial opens synchronously so the API reports the actual access error.
// EndpointSerial retries internally; the one-shot custom adapter leaves retry
// ownership with the connection service (T-049), never with gomavlib.
func OpenSerial(device Device, settings Settings, label string) (codec.FrameSource, error) {
	port, err := serial.Open(device.Path, &serial.Mode{BaudRate: settings.BaudRate, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit})
	if err != nil {
		return nil, err
	}
	conn := &serialConn{ReadWriteCloser: port}
	first := true
	node, err := codec.NewNode([]gomavlib.EndpointConf{gomavlib.EndpointCustomClient{
		Label: label,
		Connect: func(ctx context.Context) (net.Conn, error) {
			if first {
				first = false
				return conn, nil
			}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &serialSource{Node: node, conn: conn}, nil
}

type serialConn struct {
	io.ReadWriteCloser
	once sync.Once
	err  error
}

func (c *serialConn) Close() error {
	c.once.Do(func() { c.err = c.ReadWriteCloser.Close() })
	return c.err
}
func (*serialConn) LocalAddr() net.Addr              { return nil }
func (*serialConn) RemoteAddr() net.Addr             { return nil }
func (*serialConn) SetDeadline(time.Time) error      { return nil }
func (*serialConn) SetReadDeadline(time.Time) error  { return nil }
func (*serialConn) SetWriteDeadline(time.Time) error { return nil }

type serialSource struct {
	*codec.Node
	conn *serialConn
}

func (s *serialSource) Close() error {
	// Release a blocked serial read/write before waiting for gomavlib's workers.
	_ = s.conn.Close()
	return s.Node.Close()
}

func errorCode(err error) string {
	var portErr *serial.PortError
	if errors.As(err, &portErr) {
		switch portErr.Code() {
		case serial.PortBusy:
			return "busy"
		case serial.PermissionDenied:
			return "permission_denied"
		case serial.PortNotFound:
			return "missing"
		case serial.InvalidSpeed:
			return "invalid_settings"
		}
	}
	switch {
	case errors.Is(err, syscall.EBUSY):
		return "busy"
	case errors.Is(err, os.ErrPermission):
		return "permission_denied"
	case errors.Is(err, os.ErrNotExist):
		return "missing"
	default:
		return "io_error"
	}
}
