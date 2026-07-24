package zscaler

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestReadersAvoidVendoredUnboundedPagination(t *testing.T) {
	t.Parallel()

	productionFiles, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob production Go files error = %v, want nil", err)
	}
	for _, productionFile := range productionFiles {
		if strings.HasSuffix(productionFile, "_test.go") {
			continue
		}
		source, err := os.ReadFile(productionFile)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v, want nil", productionFile, err)
		}
		for _, product := range []string{"zia", "zpa", "zcc", "ztw", "zid"} {
			serviceMarker := "/zscaler/" + product + "/services/"
			if !strings.Contains(string(source), serviceMarker) {
				continue
			}

			t.Run(productionFile+"/"+product, func(t *testing.T) {
				t.Parallel()

				offenders, err := readerUnboundedSDKCalls(productionFile, product)
				if err != nil {
					t.Fatalf("readerUnboundedSDKCalls(%q) error = %v, want nil", productionFile, err)
				}
				if len(offenders) != 0 {
					t.Fatalf(
						"%s calls vendored SDK functions backed by unbounded pagination: %s",
						productionFile,
						strings.Join(offenders, ", "),
					)
				}
			})
		}
	}
}

func readerUnboundedSDKCalls(readerPath, product string) ([]string, error) {
	fileset := token.NewFileSet()
	reader, err := parser.ParseFile(fileset, readerPath, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse reader: %w", err)
	}

	serviceMarker := "/zscaler/" + product + "/services/"
	packages := make(map[string]map[string]*ast.FuncDecl)
	for _, importSpec := range reader.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("unquote import path %s: %w", importSpec.Path.Value, err)
		}
		if !strings.Contains(importPath, serviceMarker) {
			continue
		}

		vendorDir := filepath.Join("..", "..", "vendor", filepath.FromSlash(importPath))
		functions, packageName, err := parseVendorFunctions(fileset, vendorDir)
		if err != nil {
			return nil, fmt.Errorf("parse vendored package %s: %w", importPath, err)
		}

		alias := packageName
		if importSpec.Name != nil {
			alias = importSpec.Name.Name
		}
		if alias == "_" || alias == "." {
			continue
		}
		packages[alias] = functions
	}

	offenderSet := make(map[string]struct{})
	ast.Inspect(reader, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		alias, functionName, ok := selectorCall(call.Fun)
		if !ok {
			return true
		}
		functions, ok := packages[alias]
		if !ok {
			return true
		}
		function, ok := functions[functionName]
		if !ok {
			return true
		}
		if isUnboundedSDKPaginator(functionName) ||
			vendorFunctionUsesUnboundedPagination(function, functions, make(map[string]bool)) {
			offenderSet[alias+"."+functionName] = struct{}{}
		}
		return true
	})

	offenders := make([]string, 0, len(offenderSet))
	for offender := range offenderSet {
		offenders = append(offenders, offender)
	}
	sort.Strings(offenders)
	return offenders, nil
}

func parseVendorFunctions(fileset *token.FileSet, directory string) (map[string]*ast.FuncDecl, string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, "", err
	}

	functions := make(map[string]*ast.FuncDecl)
	packageName := ""
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		matches, err := build.Default.MatchFile(directory, entry.Name())
		if err != nil {
			return nil, "", fmt.Errorf("match build constraints for %s: %w", entry.Name(), err)
		}
		if !matches {
			continue
		}

		filePath := filepath.Join(directory, entry.Name())
		file, err := parser.ParseFile(fileset, filePath, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, "", fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		if packageName == "" {
			packageName = file.Name.Name
		} else if file.Name.Name != packageName {
			return nil, "", fmt.Errorf(
				"file %s has package %s, want %s",
				entry.Name(),
				file.Name.Name,
				packageName,
			)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil {
				continue
			}
			functions[function.Name.Name] = function
		}
	}
	if packageName == "" {
		return nil, "", fmt.Errorf("no buildable Go files")
	}
	return functions, packageName, nil
}

func vendorFunctionUsesUnboundedPagination(
	function *ast.FuncDecl,
	functions map[string]*ast.FuncDecl,
	visiting map[string]bool,
) bool {
	if function == nil || function.Body == nil {
		return false
	}
	if visiting[function.Name.Name] {
		return false
	}
	visiting[function.Name.Name] = true
	defer delete(visiting, function.Name.Name)

	var directCalls []string
	unbounded := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if _, calledName, ok := selectorCall(call.Fun); ok && isUnboundedSDKPaginator(calledName) {
			unbounded = true
			return false
		}
		if calledName, ok := identifierCall(call.Fun); ok {
			directCalls = append(directCalls, calledName)
		}
		return true
	})
	if unbounded {
		return true
	}
	for _, calledName := range directCalls {
		if vendorFunctionUsesUnboundedPagination(functions[calledName], functions, visiting) {
			return true
		}
	}
	return false
}

func isUnboundedSDKPaginator(name string) bool {
	return strings.HasPrefix(name, "ReadAllPages") ||
		strings.HasPrefix(name, "GetAllPagesGeneric")
}

func selectorCall(expression ast.Expr) (string, string, bool) {
	expression = unwrapCallExpression(expression)
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	return qualifier.Name, selector.Sel.Name, true
}

func identifierCall(expression ast.Expr) (string, bool) {
	expression = unwrapCallExpression(expression)
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return "", false
	}
	return identifier.Name, true
}

func unwrapCallExpression(expression ast.Expr) ast.Expr {
	for {
		switch wrapped := expression.(type) {
		case *ast.IndexExpr:
			expression = wrapped.X
		case *ast.IndexListExpr:
			expression = wrapped.X
		case *ast.ParenExpr:
			expression = wrapped.X
		default:
			return expression
		}
	}
}
