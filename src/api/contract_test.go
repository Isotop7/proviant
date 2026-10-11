// Source-tree contract tests for #396. go/parser rather than grep: splitting
// files on "func " attributes a handler to the next function's doc block, which
// reported eleven phantom Swagger mismatches.
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

// errorStatusNames are the http.StatusXxx selectors that denote a failure.
var errorStatusNames = map[string]bool{
	"StatusBadRequest":          true,
	"StatusUnauthorized":        true,
	"StatusForbidden":           true,
	"StatusNotFound":            true,
	"StatusConflict":            true,
	"StatusGone":                true,
	"StatusRequestTimeout":      true,
	"StatusUnprocessableEntity": true,
	"StatusTooManyRequests":     true,
	"StatusInternalServerError": true,
	"StatusNotImplemented":      true,
	"StatusBadGateway":          true,
	"StatusServiceUnavailable":  true,
	"StatusGatewayTimeout":      true,
}

var successStatusNames = map[string]bool{
	"StatusOK":        true,
	"StatusCreated":   true,
	"StatusAccepted":  true,
	"StatusNoContent": true,
}

var contractDirs = []string{"api", "router", "web"}

type jsonCall struct {
	relPath     string
	line        int
	status      string
	bodyName    string
	bodyType    string // resolved element type of a DTO constructor or literal body
	bodyExpr    string
	isLiteral   bool
	deliberate  bool // literal carries an Action or a shared-constant Message
	allowMarked bool
}

func selectorName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return selectorName(typed.X) + "." + typed.Sel.Name
	}
	return ""
}

// calleeName returns the final segment of a call's function name, e.g.
// "NewProductResponses" for apiModel.NewProductResponses(...).
func calleeName(expr ast.Expr) string {
	name := selectorName(expr)
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

// normalizeType drops package qualifiers so an annotation's
// "[]apiModel.ProductResponse" and a constructor's "[]ProductResponse" compare
// equal.
func normalizeType(name string) string {
	name = strings.TrimSpace(name)
	for _, qualifier := range []string{"apiModel.", "dbModel.", "database.", "authentication.", "api."} {
		name = strings.ReplaceAll(name, qualifier, "")
	}
	return name
}

// renderType renders a type expression without package qualifiers.
func renderType(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		return typed.Sel.Name
	case *ast.StarExpr:
		return "*" + renderType(typed.X)
	case *ast.ArrayType:
		return "[]" + renderType(typed.Elt)
	}
	return ""
}

// constructorReturnTypes maps each apiModel "New*" constructor to the element
// type it returns, so a body of apiModel.NewProductResponses(...) can be
// compared against the handler's @Success annotation.
func constructorReturnTypes(t *testing.T, root string) map[string]string {
	t.Helper()
	types := map[string]string{}
	fset := token.NewFileSet()
	dir := filepath.Join(root, "models", "api")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", entry.Name(), parseErr)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !strings.HasPrefix(fn.Name.Name, "New") || fn.Type.Results == nil {
				continue
			}
			if len(fn.Type.Results.List) != 1 {
				continue
			}
			types[fn.Name.Name] = renderType(fn.Type.Results.List[0].Type)
		}
	}
	return types
}

// literalIsCompliant reports whether an APIResponse error literal is deliberate:
// it carries an Action hint, or its Message is not a bare string literal. A
// computed message (fmt.Sprintf, err.Error()) counts as deliberate — the callers
// that use one echo a caller-supplied value (barcode, limit) that no constant
// can carry. The rule bans the hardcoded literal, not computation.
func literalIsCompliant(lit *ast.CompositeLit) bool {
	hasAction := false
	messageIsComputed := false
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Action":
			hasAction = true
		case "Message":
			if _, isLit := kv.Value.(*ast.BasicLit); !isLit {
				messageIsComputed = true
			}
		}
	}
	return hasAction || messageIsComputed
}

// hasAllowMarker reports whether a "//contract:allow <reason>" comment sits above
// the call, so an exemption stays deliberate and greppable.
func hasAllowMarker(lines []string, line int) bool {
	idx := line - 2
	if idx < 0 {
		return false
	}
	return strings.Contains(lines[idx], "//contract:allow")
}

// collectJSONCalls returns every ctx.JSON call under the handler trees. The
// constructor map resolves apiModel.NewX(...) bodies to their element type.
func collectJSONCalls(t *testing.T, constructors map[string]string) (calls []jsonCall, docs map[string]string) {
	t.Helper()
	root := repoRoot(t)
	fset := token.NewFileSet()
	docs = map[string]string{}

	for _, dir := range contractDirs {
		dirPath := filepath.Join(root, dir)
		if _, err := os.Stat(dirPath); err != nil {
			t.Fatalf("expected handler tree %s: %v", dirPath, err)
		}

		err := filepath.WalkDir(dirPath, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "testutil" || entry.Name() == "mocks" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			parsed, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if parseErr != nil {
				return parseErr
			}
			source, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			lines := strings.Split(string(source), "\n")
			rel, _ := filepath.Rel(root, path)

			for _, decl := range parsed.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Doc == nil {
					continue
				}
				funcLine := fset.Position(fn.Pos()).Line
				docs[rel+":"+strconv.Itoa(funcLine)] = fn.Doc.Text()
			}

			ast.Inspect(parsed, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "JSON" || len(call.Args) < 2 {
					return true
				}
				status, isStatus := strings.CutPrefix(selectorName(call.Args[0]), "http.")
				if !isStatus {
					return true
				}

				bodyName := selectorName(call.Args[1])
				bodyType := ""
				isLiteral, compliant := false, false
				switch typed := call.Args[1].(type) {
				case *ast.CompositeLit:
					bodyName = selectorName(typed.Type)
					isLiteral = true
					compliant = literalIsCompliant(typed)
				case *ast.CallExpr:
					bodyType = constructors[calleeName(typed.Fun)]
				}

				line := fset.Position(call.Pos()).Line
				calls = append(calls, jsonCall{
					relPath:     rel,
					line:        line,
					status:      status,
					bodyName:    bodyName,
					bodyType:    bodyType,
					bodyExpr:    exprString(call.Args[1]),
					isLiteral:   isLiteral,
					deliberate:  compliant,
					allowMarked: hasAllowMarker(lines, line),
				})
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dirPath, err)
		}
	}
	return calls, docs
}

func exprString(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.CompositeLit:
		return selectorName(typed.Type) + "{...}"
	case *ast.Ident:
		return typed.Name
	case *ast.CallExpr:
		return selectorName(typed.Fun) + "(...)"
	}
	return "?"
}

// enclosingHandler returns the doc of the nearest preceding func in the file.
func enclosingHandler(rel string, line int, funcLines map[string][]int, docs map[string]string) string {
	candidates := funcLines[rel]
	best := -1
	for _, fl := range candidates {
		if fl <= line && fl > best {
			best = fl
		}
	}
	if best < 0 {
		return ""
	}
	return docs[rel+":"+strconv.Itoa(best)]
}

func funcLineIndex(root string) map[string][]int {
	index := map[string][]int{}
	fset := token.NewFileSet()
	for _, dir := range contractDirs {
		filepath.WalkDir(filepath.Join(root, dir), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			parsed, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			for _, decl := range parsed.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok {
					index[rel] = append(index[rel], fset.Position(fn.Pos()).Line)
				}
			}
			return nil
		})
	}
	return index
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate go.mod above the api package")
	return ""
}

var successAnnotation = regexp.MustCompile(`@Success\s+(\S+)\s+\{([^}]*)\}\s+(\S+)`)

// documentedType renders a @Success type token in the same shape the handler
// bodies are compared in, following how swaggo actually reads the annotation:
// an "{array}" kind makes the documented type a slice of the token, while an
// "{object}" kind takes the token verbatim. So "{array} apiModel.X" and
// "{object} []apiModel.X" both document []X — and "{array} []apiModel.X", which
// swaggo would render as a slice of slices, documents [][]X and is caught here.
func documentedType(kind, typeName string) string {
	normalized := normalizeType(typeName)
	if strings.Contains(kind, "array") {
		return "[]" + normalized
	}
	return normalized
}

// isSuccessCode reports whether a @Success code denotes a 2xx. Annotations spell
// the status numerically, unlike the http.StatusXxx selectors in handler bodies.
func isSuccessCode(code string) bool {
	numeric, err := strconv.Atoi(strings.TrimPrefix(code, "http."))
	if err != nil {
		return successStatusNames[strings.TrimPrefix(code, "http.")]
	}
	return numeric >= 200 && numeric < 300
}

// TestNoInlineErrorBodies fails if a handler builds an error response from a bare
// string literal. Literals carrying an Action hint, and computed Messages that
// echo a caller-supplied value, are deliberate and exempt.
func TestNoInlineErrorBodies(t *testing.T) {
	calls, _ := collectJSONCalls(t, nil)

	var offenders []string
	for _, call := range calls {
		if !errorStatusNames[call.status] || call.allowMarked {
			continue
		}
		if call.bodyName != "api.APIResponse" && call.bodyName != "APIResponse" {
			continue
		}
		if !call.isLiteral || call.deliberate {
			continue
		}
		offenders = append(offenders, call.relPath+":"+strconv.Itoa(call.line))
	}

	if len(offenders) > 0 {
		t.Errorf("%d error responses build their body from a bare literal; use api.RespondError, api.InternalError, api.InvalidInputError or a sentinel from errors (or add //contract:allow <reason> above):\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}

// TestSwaggerSuccessMatchesHandler fails if a handler's @Success disagrees with
// the body it returns — Swagger is the only public description of this API. A
// resolvable DTO body (an apiModel constructor or a literal) must match the
// documented element type exactly; a variable body falls back to checking only
// that the envelope shape agrees.
func TestSwaggerSuccessMatchesHandler(t *testing.T) {
	root := repoRoot(t)
	constructors := constructorReturnTypes(t, root)
	calls, docs := collectJSONCalls(t, constructors)
	funcLines := funcLineIndex(root)

	var offenders []string
	for _, call := range calls {
		if !successStatusNames[call.status] {
			continue
		}
		doc := enclosingHandler(call.relPath, call.line, funcLines, docs)
		if doc == "" || !strings.Contains(doc, "@Success") {
			continue
		}

		var documentedTypes []string
		documentedAsEnvelope := false
		for _, line := range strings.Split(doc, "\n") {
			matches := successAnnotation.FindStringSubmatch(line)
			if matches == nil || !isSuccessCode(matches[1]) {
				continue
			}
			documentedTypes = append(documentedTypes, documentedType(matches[2], matches[3]))
			if strings.HasSuffix(matches[3], "api.APIResponse") {
				documentedAsEnvelope = true
			}
		}
		if len(documentedTypes) == 0 {
			continue
		}

		// A non-envelope DTO body must match the documented type exactly. This
		// is what catches a handler returning apiModel.NewThing(...) while
		// still documenting database.Thing.
		if call.bodyType != "" && !strings.HasSuffix(call.bodyType, "APIResponse") {
			if !containsString(documentedTypes, call.bodyType) {
				offenders = append(offenders, call.relPath+":"+strconv.Itoa(call.line))
			}
			continue
		}

		returnsEnvelope := strings.HasSuffix(call.bodyName, "APIResponse")
		if documentedAsEnvelope != returnsEnvelope {
			offenders = append(offenders, call.relPath+":"+strconv.Itoa(call.line))
		}
	}

	if len(offenders) > 0 {
		t.Errorf("%d handlers document a @Success that disagrees with the body they return:\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}

func containsString(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}
