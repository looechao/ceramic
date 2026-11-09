package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// These boundaries are easy to accidentally cross while adding an event handler.
func TestLayerDependencies(t *testing.T) {
	for _, pattern := range []string{"*_vm.go", "*_ui.go", "../draftstore/*.go", "../preferences/*.go"} {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			vm := strings.HasSuffix(path, "_vm.go")
			view := strings.HasSuffix(path, "_ui.go")
			for _, imp := range file.Imports {
				name, _ := strconv.Unquote(imp.Path.Value)
				if !view && strings.HasPrefix(name, "github.com/egoist/mygo") {
					t.Errorf("%s imports MyGo", path)
				}
				if !vm && !view && strings.HasPrefix(name, "ceramic/") {
					t.Errorf("%s depends on application code", path)
				}
				if view && (name == "os" || name == "ceramic/internal/preferences" || name == "ceramic/internal/draftstore") {
					t.Errorf("%s accesses persistence", path)
				}
			}
			ast.Inspect(file, func(n ast.Node) bool {
				if selector, ok := n.(*ast.SelectorExpr); view && ok && selector.Sel.Name == "service" {
					t.Errorf("%s directly accesses protocol service", path)
				}
				if id, ok := n.(*ast.Ident); vm && ok && id.Name == "reader" {
					t.Errorf("%s depends on view host", path)
				}
				return true
			})
		}
	}
}
