// SPDX-License-Identifier: GPL-3.0-only

package cubicstart_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestInternalPackageEntrypoints(t *testing.T) {
	checked := make(map[string]bool)
	err := filepath.WalkDir("internal", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		dir := filepath.Dir(path)
		name := filepath.Base(dir)
		publicFile := filepath.Join(dir, "interface.go")
		if !checked[dir] {
			checked[dir] = true
			info, err := os.Stat(publicFile)
			if err != nil {
				t.Errorf("%s: required entrypoint %s: %v", dir, publicFile, err)
			} else if !info.Mode().IsRegular() {
				t.Errorf("%s: entrypoint must be a regular file", publicFile)
			}
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		if file.Name.Name != name {
			t.Errorf("%s: package must be named %s", path, name)
		}
		if path != publicFile {
			for _, ident := range publicDeclarations(file) {
				t.Errorf("%s: move public declaration %s to %s", fset.Position(ident.Pos()), ident.Name, publicFile)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCoreImportsOnlyStandardLibrary(t *testing.T) {
	err := filepath.WalkDir(filepath.Join("internal", "core"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			// Standard-library import paths have no domain in their first component.
			first, _, _ := strings.Cut(importPath, "/")
			if strings.Contains(first, ".") || strings.HasPrefix(importPath, "github.com/Evernight275/cubicstart/") {
				t.Errorf("%s: core must not depend on implementation or third-party package %s", path, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func publicDeclarations(file *ast.File) []*ast.Ident {
	var names []*ast.Ident
	add := func(name *ast.Ident) {
		if name.IsExported() {
			names = append(names, name)
		}
	}
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Recv == nil || exportedReceiver(decl.Recv.List[0].Type) {
				add(decl.Name)
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					add(spec.Name)
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						add(name)
					}
				}
			}
		}
	}
	return names
}

func exportedReceiver(expr ast.Expr) bool {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.IsExported()
	case *ast.StarExpr:
		return exportedReceiver(expr.X)
	case *ast.IndexExpr:
		return exportedReceiver(expr.X)
	case *ast.IndexListExpr:
		return exportedReceiver(expr.X)
	default:
		return false
	}
}

func TestPublicDeclarations(t *testing.T) {
	cases := []struct {
		name string
		code string
		want string
	}{
		{"interface", "type Service interface { Run() }", "Service"},
		{"type alias", "type Result = string", "Result"},
		{"function", "func New() {}", "New"},
		{"values", "var Public, private int; const Limit = 1", "Public,Limit"},
		{"public receiver", "type Service struct{}; func (Service) Run() {}", "Service,Run"},
		{"generic receiver", "type Service[T any] struct{}; func (*Service[T]) Run() {}", "Service,Run"},
		{"implementation", "type service struct{}; func (*service) Run() {}; func helper() {}", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "implementation.go", "package example\n"+tc.code, 0)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, ident := range publicDeclarations(file) {
				got = append(got, ident.Name)
			}
			if strings.Join(got, ",") != tc.want {
				t.Errorf("public declarations = %v, want %q", got, tc.want)
			}
		})
	}
}
