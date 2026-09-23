package connection

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"go.bug.st/serial/enumerator"
)

// NativeInventory uses OS metadata. ExtraPaths explicitly admits non-enumerated
// UARTs and pseudo-terminals; only existing character devices are candidates.
// It never assigns a radio model based on a generic USB/serial chip identity.
type NativeInventory struct{ ExtraPaths []string }

func (i NativeInventory) List(ctx context.Context) ([]Device, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}
	out := make([]Device, 0, len(ports))
	seen := map[string]bool{}
	for _, p := range ports {
		path, err := filepath.EvalSymlinks(p.Name)
		if err != nil {
			continue
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, Device{ID: "serial:" + path, Kind: "serial", Path: path,
			Description: p.Product, SerialNumber: p.SerialNumber, Manufacturer: p.Manufacturer, VID: p.VID, PID: p.PID})
	}
	for _, name := range i.ExtraPaths {
		path, err := filepath.EvalSymlinks(name)
		if err != nil {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode()&os.ModeCharDevice == 0 || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, Device{ID: "serial:" + path, Kind: "serial", Path: path})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out, nil
}
