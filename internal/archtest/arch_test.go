// Package archtest keeps the hexagon honest by inspecting imports (standard library only).
package archtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/cbuelvasc/orders-mcp-go"

// imports returns, for every non-test Go file, its package dir (relative to the repo) and imports.
func imports(t *testing.T) map[string][]string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	out := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			out[filepath.ToSlash(rel)] = append(out[filepath.ToSlash(rel)], p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func isStdlib(p string) bool { return !strings.Contains(strings.Split(p, "/")[0], ".") }

func check(t *testing.T, rule string, applies func(pkg string) bool, forbidden func(imp string) bool) {
	t.Helper()
	for pkg, imps := range imports(t) {
		if !applies(pkg) {
			continue
		}
		for _, imp := range imps {
			if forbidden(imp) {
				t.Errorf("%s: %s imports %s", rule, pkg, imp)
			}
		}
	}
}

func under(prefix string) func(string) bool {
	return func(pkg string) bool { return pkg == prefix || strings.HasPrefix(pkg, prefix+"/") }
}

func TestDomainIsPureGo(t *testing.T) {
	check(t, "domain must only use the standard library", under("internal/domain"),
		func(imp string) bool { return !isStdlib(imp) })
}

func TestApplicationOnlyKnowsTheDomain(t *testing.T) {
	check(t, "app may only import the domain", under("internal/app"),
		func(imp string) bool { return !isStdlib(imp) && imp != module+"/internal/domain" })
}

func TestOnlyTheMcpAdapterAndMainKnowTheSDK(t *testing.T) {
	check(t, "MCP SDK outside its adapter",
		func(pkg string) bool { return pkg != "internal/adapter/in/mcp" && pkg != "cmd/server" },
		func(imp string) bool { return strings.HasPrefix(imp, "github.com/modelcontextprotocol/go-sdk") })
}

func TestOnlyThePostgresAdapterAndMainKnowSQL(t *testing.T) {
	check(t, "pgx outside its adapter",
		func(pkg string) bool { return pkg != "internal/adapter/out/postgres" && pkg != "cmd/server" },
		func(imp string) bool { return strings.HasPrefix(imp, "github.com/jackc/pgx") })
}

func TestInboundAdaptersTalkToPortsNotToServices(t *testing.T) {
	check(t, "inbound adapter depends on app", under("internal/adapter/in"),
		func(imp string) bool { return imp == module+"/internal/app" })
}
