package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var routeMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}

// routeDocExempt holds routes served by inline or static handlers that carry no
// Swagger operation because they never return an API payload.
var routeDocExempt = map[string]bool{
	"get /":              true,
	"get /favicon.ico":   true,
	"get /manifest.json": true,
	"get /sw.js":         true,
}

var routerAnnotation = regexp.MustCompile(`@Router\s+(\S+)\s+\[(\w+)\]`)
var ginPathParam = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

// TestEveryRouteHasRouterAnnotation fails when a route registered in the router
// has no matching @Router annotation, so generated Swagger cannot silently drop
// a route again.
func TestEveryRouteHasRouterAnnotation(t *testing.T) {
	root := repoRoot(t)
	routes := registeredRoutes(t, root)
	documented := documentedRoutes(t, root)

	var missing []string
	for _, route := range routes {
		if routeDocExempt[route] {
			continue
		}
		if !documented[route] {
			missing = append(missing, route)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d routes are missing a @Router annotation (or the annotation path does not match):\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}

	var stale []string
	registered := map[string]bool{}
	for _, route := range routes {
		registered[route] = true
	}
	for route := range documented {
		if !registered[route] {
			stale = append(stale, route)
		}
	}
	if len(stale) > 0 {
		t.Errorf("%d @Router annotations do not match any registered route:\n  %s",
			len(stale), strings.Join(stale, "\n  "))
	}
}

// registeredRoutes returns "method /path" for every route registration in
// router.go, resolving group prefixes and util.RouteXxx path constants.
func registeredRoutes(t *testing.T, root string) []string {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filepath.Join(root, "router", "router.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse router.go: %v", err)
	}

	constants := utilStringConstants(t, root)
	prefixes := map[string]string{"engine": ""}

	var routes []string
	ast.Inspect(parsed, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		lhs, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Group" || len(call.Args) < 1 {
			return true
		}
		parent, ok := prefixes[selectorName(sel.X)]
		if !ok {
			t.Errorf("group %s derives from unknown router %s", lhs.Name, selectorName(sel.X))
			return true
		}
		prefixes[lhs.Name] = parent + literalString(t, call.Args[0], constants)
		return true
	})

	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !routeMethods[sel.Sel.Name] || len(call.Args) < 1 {
			return true
		}
		prefix, ok := prefixes[selectorName(sel.X)]
		if !ok {
			t.Errorf("route registered on unknown router %s", selectorName(sel.X))
			return true
		}
		route := normalizeRoutePath(prefix + literalString(t, call.Args[0], constants))
		routes = append(routes, strings.ToLower(sel.Sel.Name)+" "+route)
		return true
	})
	return routes
}

// documentedRoutes returns "method /path" for every @Router annotation in the
// source tree.
func documentedRoutes(t *testing.T, root string) map[string]bool {
	t.Helper()
	documented := map[string]bool{}
	fset := token.NewFileSet()
	for _, dir := range contractDirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			parsed, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if parseErr != nil {
				return parseErr
			}
			for _, decl := range parsed.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Doc == nil {
					continue
				}
				for _, match := range routerAnnotation.FindAllStringSubmatch(fn.Doc.Text(), -1) {
					documented[strings.ToLower(match[2])+" "+normalizeRoutePath(match[1])] = true
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	return documented
}

// utilStringConstants maps each util.RouteXxx style constant to its value so
// non-literal route paths resolve.
func utilStringConstants(t *testing.T, root string) map[string]string {
	t.Helper()
	constants := map[string]string{}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(filepath.Join(root, "util"))
	if err != nil {
		t.Fatalf("read util: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fset, filepath.Join(root, "util", entry.Name()), nil, 0)
		if parseErr != nil {
			t.Fatalf("parse util/%s: %v", entry.Name(), parseErr)
		}
		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				values, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range values.Names {
					if i >= len(values.Values) {
						continue
					}
					lit, ok := values.Values[i].(*ast.BasicLit)
					if ok && lit.Kind == token.STRING {
						constants[name.Name], _ = strconv.Unquote(lit.Value)
					}
				}
			}
		}
	}
	return constants
}

func literalString(t *testing.T, expr ast.Expr, constants map[string]string) string {
	t.Helper()
	switch typed := expr.(type) {
	case *ast.BasicLit:
		value, err := strconv.Unquote(typed.Value)
		if err != nil {
			t.Fatalf("unquote route path %s: %v", typed.Value, err)
		}
		return value
	case *ast.SelectorExpr:
		value, ok := constants[typed.Sel.Name]
		if !ok {
			t.Fatalf("unknown route path constant %s", selectorName(typed))
		}
		return value
	}
	t.Fatalf("unsupported route path expression %T", expr)
	return ""
}

// normalizeRoutePath renders gin's :param syntax in swag's {param} form and
// drops the trailing slash gin tolerates, so both sides compare equal.
func normalizeRoutePath(path string) string {
	normalized := ginPathParam.ReplaceAllString(path, "{$1}")
	if len(normalized) > 1 {
		normalized = strings.TrimSuffix(normalized, "/")
	}
	return normalized
}
