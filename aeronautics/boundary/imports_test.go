// Package boundary_test checks architecture from outside the pure core.
package boundary_test

import (
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
)

type listedPackage struct {
	Module     *struct{ Path string }
	ImportPath string
	Standard   bool
}

func TestCalculatorTransitiveImports(t *testing.T) {
	// Expand the named stdlib allowlist to include its implementation dependencies
	// for the active Go toolchain. Explicit I/O/clock bans take precedence.
	allowed := make(map[string]bool)
	for _, pkg := range listPackages(t, "math", "fmt", "errors", "strconv", "sort", "testing") {
		allowed[pkg.ImportPath] = true
	}
	for _, pkg := range listPackages(t, "yalb.aero/calculator") {
		if reason := importIssue(pkg, allowed); reason != "" {
			t.Errorf("core dependency %s: %s", pkg.ImportPath, reason)
		}
	}
}

func TestImportPolicy(t *testing.T) {
	allowed := map[string]bool{"math": true, "internal/cpu": true, "time": true}
	for _, path := range []string{"net/http", "net", "os", "os/exec", "database/sql", "time"} {
		t.Run(path, func(t *testing.T) {
			// Even membership in the expanded allowlist cannot permit these.
			allowed[path] = true
			if importIssue(listedPackage{ImportPath: path, Standard: true}, allowed) == "" {
				t.Fatalf("forbidden dependency %s accepted", path)
			}
		})
	}
	for _, path := range []string{"math", "internal/cpu"} {
		if issue := importIssue(listedPackage{ImportPath: path, Standard: true}, allowed); issue != "" {
			t.Errorf("allowed dependency %s rejected: %s", path, issue)
		}
	}
	for _, pkg := range []listedPackage{
		{ImportPath: "crypto/sha256", Standard: true},
		{ImportPath: "yalb.gcs/internal/vehicle", Module: &struct{ Path string }{Path: "yalb.gcs"}},
		{ImportPath: "example.org/transport", Module: &struct{ Path string }{Path: "example.org/transport"}},
	} {
		if importIssue(pkg, allowed) == "" {
			t.Errorf("unapproved dependency %s accepted", pkg.ImportPath)
		}
	}
}

func importIssue(pkg listedPackage, allowed map[string]bool) string {
	for _, banned := range []string{"net", "os", "database/sql", "time"} {
		if pkg.ImportPath == banned || strings.HasPrefix(pkg.ImportPath, banned+"/") {
			return "transport, storage, filesystem and clock access must stay outside calculator"
		}
	}
	if pkg.Module != nil && pkg.Module.Path == "yalb.aero" {
		return ""
	}
	if pkg.Standard && allowed[pkg.ImportPath] {
		return ""
	}
	return "dependency is outside yalb.aero and the approved standard library closure"
}

func listPackages(t *testing.T, patterns ...string) []listedPackage {
	t.Helper()
	args := append([]string{"list", "-deps", "-json"}, patterns...)
	cmd := exec.CommandContext(t.Context(), "go", args...)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list %v: %v", patterns, err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var packages []listedPackage
	for {
		var pkg listedPackage
		if err := decoder.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				return packages
			}
			t.Fatalf("decode go list: %v", err)
		}
		packages = append(packages, pkg)
	}
}
