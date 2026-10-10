//nolint:testpackage // GO-15: compares the private admission, constructor and media type switches structurally.
package webtransport

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestAuthenticationAdmissionAndConstructionCoverTheSameCalls(t *testing.T) {
	t.Parallel()

	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("authentication contract test source path is unavailable")
	}

	root := filepath.Dir(source)
	admitted := authenticationTypeCases(t, filepath.Join(root, "auth_exchange.go"), "ExchangeAuthentication")
	constructed := authenticationTypeCases(t, filepath.Join(root, "auth_call_requests.go"), "buildAuthenticationRequest")
	media := authenticationTypeCases(t, filepath.Join(root, "auth_call_media.go"), "authenticationCallMedia")

	if len(admitted) == 0 || !reflect.DeepEqual(admitted, constructed) || !reflect.DeepEqual(admitted, media) {
		t.Fatalf("authentication admission, construction, and media differ: admission=%v construction=%v media=%v",
			admitted, constructed, media)
	}
}

func authenticationTypeCases(t *testing.T, path, functionName string) map[string]bool {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	result := map[string]bool{}

	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != functionName {
			continue
		}

		ast.Inspect(function.Body, func(node ast.Node) bool {
			clause, ok := node.(*ast.CaseClause)
			if !ok {
				return true
			}

			for _, expression := range clause.List {
				name, named := expression.(*ast.Ident)
				if !named {
					t.Fatalf("authentication admission contains a non-concrete type: %T", expression)
				}

				if result[name.Name] {
					t.Fatalf("duplicate authentication call: %s", name.Name)
				}

				result[name.Name] = true
			}

			return false
		})
	}

	return result
}
