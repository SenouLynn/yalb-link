// Package boundary_test checks architecture from outside the pure core.
package boundary_test

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
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

// productionSeed is the stdlib the calculator core may reach. The allowlist is
// the closure of these packages, because literal paths alone would reject the
// Go toolchain's own implementation packages such as internal/cpu.
//
// Task 01 names six packages: math, fmt, errors, strconv, sort and testing.
// Two of them are deliberately not seeded here:
//
//   - testing is a dependency of the tests, not of the core. This check reads
//     the non-test closure, where testing has no business appearing. Seeding it
//     expanded its closure into the production allowlist, which quietly admitted
//     strings, bytes, io, reflect, context, sync/atomic, flag and filepath.
//   - fmt reaches os, which the same task bans, so the core formats with strconv
//     instead. Seeding a package whose closure is then rejected by the bans
//     buys nothing and widens the allowlist by everything else fmt pulls in.
//
// Seeding four instead of six takes the allowlist from 71 packages to 38 and
// leaves the core's actual closure a clean subset. Widening this list is a
// decision to be made on purpose, which is the whole point of the check.
var productionSeed = []string{"math", "errors", "strconv", "sort"}

func TestCalculatorTransitiveImports(t *testing.T) {
	// Expand the seed to include its implementation dependencies for the active
	// Go toolchain. Explicit I/O/clock bans take precedence.
	allowed := make(map[string]bool)
	for _, pkg := range listPackages(t, productionSeed...) {
		allowed[pkg.ImportPath] = true
	}
	for _, pkg := range listPackages(t, "yalb.aero/calculator") {
		if reason := importIssue(pkg, allowed); reason != "" {
			t.Errorf("core dependency %s: %s", pkg.ImportPath, reason)
		}
	}
}

// TestAllowlistDoesNotAdmitTestOnlyPackages pins the seed narrowing. These
// packages are not banned outright — a sibling package outside the core may
// legitimately use any of them — but none may enter the production core by
// riding in on another package's closure. Re-seeding testing or fmt fails this.
func TestAllowlistDoesNotAdmitTestOnlyPackages(t *testing.T) {
	allowed := make(map[string]bool)
	for _, pkg := range listPackages(t, productionSeed...) {
		allowed[pkg.ImportPath] = true
	}
	for _, path := range []string{
		"strings", "bytes", "io", "reflect", "context",
		"sync", "sync/atomic", "flag", "path/filepath", "runtime/debug",
	} {
		t.Run(path, func(t *testing.T) {
			if importIssue(listedPackage{ImportPath: path, Standard: true}, allowed) == "" {
				t.Errorf("%s is admitted into the production core by closure expansion", path)
			}
		})
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

// TestApplicationBoundaryDoesNotDependOnHTTP holds the seam Task 05 is about:
// the application boundary is transport-neutral, so an MCP sidecar can reuse it
// directly instead of making loopback HTTP calls, and the calculation core does
// not know either of them exists.
func TestApplicationBoundaryDoesNotDependOnHTTP(t *testing.T) {
	for _, tc := range []struct {
		pkg       string
		forbidden []string
	}{
		{
			pkg:       "yalb.aero/api",
			forbidden: []string{"net/http", "net", "yalb.aero/httpapi", "os", "database/sql"},
		},
		{
			pkg:       "yalb.aero/calculator",
			forbidden: []string{"yalb.aero/api", "yalb.aero/httpapi", "encoding/json", "context"},
		},
	} {
		t.Run(tc.pkg, func(t *testing.T) {
			banned := make(map[string]bool, len(tc.forbidden))
			for _, path := range tc.forbidden {
				banned[path] = true
			}
			for _, pkg := range listPackages(t, tc.pkg) {
				if banned[pkg.ImportPath] {
					t.Errorf("%s reaches %s, which its seam forbids", tc.pkg, pkg.ImportPath)
				}
			}
		})
	}
}

// TestTransportDoesNotReimplementTheCore holds that the boundary packages carry
// no arithmetic. Multiplication, division, subtraction and remainder are how a
// unit conversion or a formula gets copied into a transport, where it would then
// be free to disagree with the core; string concatenation and comparison stay
// allowed because those are what building a message and checking a shape are
// made of.
//
// Every number a response carries therefore came out of yalb.aero/calculator.
func TestTransportDoesNotReimplementTheCore(t *testing.T) {
	for _, dir := range []string{"../api", "../httpapi"} {
		t.Run(dir, func(t *testing.T) {
			fset := token.NewFileSet()
			packages, err := parser.ParseDir(fset, dir, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", dir, err)
			}
			for _, pkg := range packages {
				for name, file := range pkg.Files {
					if strings.HasSuffix(name, "_test.go") {
						continue
					}
					checkNoArithmetic(t, fset, file)
				}
			}
		})
	}
}

func checkNoArithmetic(t *testing.T, fset *token.FileSet, file *ast.File) {
	t.Helper()
	ast.Inspect(file, func(node ast.Node) bool {
		binary, ok := node.(*ast.BinaryExpr)
		if !ok {
			return true
		}
		switch binary.Op {
		case token.MUL, token.QUO, token.SUB, token.REM:
			t.Errorf("%s: %s is arithmetic; every number the boundary reports must come from the core",
				fset.Position(binary.Pos()), binary.Op)
		default:
		}
		return true
	})
}
