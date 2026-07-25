package contracts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var lowerCamelName = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)

// TestAPINamesAreLowerCamelCase makes API casing a build-time invariant.
// Database and ClickHouse tags are deliberately out of scope: they describe
// storage schemas, not the HTTP contract.
func TestAPINamesAreLowerCamelCase(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate naming test")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	fset := token.NewFileSet()

	for _, sourceDir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, sourceDir), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.Contains(path, "llmproviders") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(node ast.Node) bool {
				switch value := node.(type) {
				case *ast.Field:
					checkJSONTag(t, fset, value)
				case *ast.CallExpr:
					checkQueryParameter(t, fset, value)
				case *ast.IndexExpr:
					checkDirectQueryLookup(t, fset, value)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", sourceDir, err)
		}
	}
}

func checkJSONTag(t *testing.T, fset *token.FileSet, field *ast.Field) {
	if field.Tag == nil {
		return
	}
	raw, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return
	}
	name := strings.Split(reflect.StructTag(raw).Get("json"), ",")[0]
	if name == "" || name == "-" || lowerCamelName.MatchString(name) {
		return
	}
	t.Errorf("%s: JSON field %q must be lower camelCase", fset.Position(field.Tag.Pos()), name)
}

func checkQueryParameter(t *testing.T, fset *token.FileSet, call *ast.CallExpr) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	name := selector.Sel.Name
	if name == "Get" && len(call.Args) > 0 {
		queryCall, ok := selector.X.(*ast.CallExpr)
		if ok {
			querySelector, ok := queryCall.Fun.(*ast.SelectorExpr)
			if ok && querySelector.Sel.Name == "Query" {
				checkStringName(t, fset, call.Args[0], "query parameter")
			}
		}
		return
	}
	if name == "FormValue" && len(call.Args) > 0 {
		checkStringName(t, fset, call.Args[0], "query parameter")
		return
	}
	if !strings.HasPrefix(name, "Parse") || !strings.HasSuffix(name, "Param") || len(call.Args) < 2 {
		return
	}
	checkStringName(t, fset, call.Args[1], "query parameter")
}

func checkDirectQueryLookup(t *testing.T, fset *token.FileSet, index *ast.IndexExpr) {
	call, ok := index.X.(*ast.CallExpr)
	if !ok {
		return
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Query" {
		return
	}
	checkStringName(t, fset, index.Index, "query parameter")
}

func checkStringName(t *testing.T, fset *token.FileSet, expr ast.Expr, kind string) {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return
	}
	name, err := strconv.Unquote(literal.Value)
	if err != nil || name == "" || lowerCamelName.MatchString(name) {
		return
	}
	t.Errorf("%s: %s %q must be lower camelCase", fset.Position(expr.Pos()), kind, name)
}
