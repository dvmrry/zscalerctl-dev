//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

type goPackage struct {
	Dir        string   `json:"Dir"`
	ImportPath string   `json:"ImportPath"`
	Name       string   `json:"Name"`
	GoFiles    []string `json:"GoFiles"`
}

type parsedPackage struct {
	meta    goPackage
	structs map[string]structInfo
}

type structInfo struct {
	file    *ast.File
	imports map[string]string
	typ     *ast.StructType
}

type sdkField struct {
	GoName    string
	JSONName  string
	Type      string
	Fields    []sdkField
	Opaque    bool
	TypeRef   typeRef
	Primitive bool
}

type typeRef struct {
	PackagePath string
	PackageName string
	TypeName    string
}

type inspector struct {
	packages map[string]*parsedPackage
}

func main() {
	var packagePath string
	var typeName string
	var product string
	var resource string
	flag.StringVar(&packagePath, "package", "", "Go package import path or relative package path containing the SDK struct")
	flag.StringVar(&typeName, "type", "", "SDK struct type name")
	flag.StringVar(&product, "product", "", "catalog product, currently zia or zpa")
	flag.StringVar(&resource, "resource", "", "catalog resource name")
	flag.Parse()

	if packagePath == "" || typeName == "" || product == "" || resource == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/catalog-draft.go --package PKG --type TYPE --product zia|zpa --resource NAME")
		os.Exit(2)
	}
	if productConst(product) == "" {
		fmt.Fprintf(os.Stderr, "catalog-draft: unsupported product %q; want zia or zpa\n", product)
		os.Exit(2)
	}
	if err := validatePackagePath(packagePath); err != nil {
		fmt.Fprintf(os.Stderr, "catalog-draft: %v\n", err)
		os.Exit(2)
	}

	pkg, err := loadPackage(packagePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "catalog-draft: load package: %v\n", err)
		os.Exit(1)
	}
	inspector := newInspector()
	fields, err := inspector.exportedJSONFields(pkg.ImportPath, typeName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "catalog-draft: inspect %s.%s: %v\n", packagePath, typeName, err)
		os.Exit(1)
	}
	writeDraft(os.Stdout, pkg, typeName, product, resource, fields)
}

func newInspector() *inspector {
	return &inspector{packages: map[string]*parsedPackage{}}
}

func loadPackage(packagePath string) (goPackage, error) {
	cmd := exec.Command("go", "list", "-json", "-mod=mod", packagePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return goPackage{}, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var pkg goPackage
	if err := json.Unmarshal(out, &pkg); err != nil {
		return goPackage{}, err
	}
	if pkg.Dir == "" {
		return goPackage{}, errors.New("go list returned empty package dir")
	}
	return pkg, nil
}

func validatePackagePath(packagePath string) error {
	if strings.HasPrefix(packagePath, "github.com/zscaler/zscaler-sdk-go/v3/") {
		return nil
	}
	if strings.HasPrefix(packagePath, "./") {
		return nil
	}
	return fmt.Errorf("package %q is outside the Zscaler SDK module", packagePath)
}

func importMap(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, item := range file.Imports {
		path, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			continue
		}
		if item.Name != nil && item.Name.Name != "" && item.Name.Name != "_" && item.Name.Name != "." {
			imports[item.Name.Name] = path
			continue
		}
		imports[importName(path)] = path
	}
	return imports
}

func importName(path string) string {
	index := strings.LastIndexByte(path, '/')
	if index >= 0 {
		return path[index+1:]
	}
	return path
}

func copyStack(stack map[string]bool) map[string]bool {
	out := make(map[string]bool, len(stack)+1)
	for key, value := range stack {
		out[key] = value
	}
	return out
}

func (i *inspector) exportedJSONFields(packagePath, typeName string) ([]sdkField, error) {
	return i.exportedJSONFieldsFrom(packagePath, typeName, map[string]bool{})
}

func (i *inspector) exportedJSONFieldsFrom(packagePath, typeName string, stack map[string]bool) ([]sdkField, error) {
	pkg, err := i.parsePackage(packagePath)
	if err != nil {
		return nil, err
	}
	key := packagePath + "." + typeName
	if stack[key] {
		return nil, nil
	}
	info, ok := pkg.structs[typeName]
	if !ok {
		return nil, fmt.Errorf("type %s not found", typeName)
	}
	nextStack := copyStack(stack)
	nextStack[key] = true
	fields, err := i.structJSONFields(pkg, info, nextStack)
	if err != nil {
		return nil, err
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].JSONName < fields[j].JSONName
	})
	return fields, nil
}

func (i *inspector) parsePackage(packagePath string) (*parsedPackage, error) {
	if pkg, ok := i.packages[packagePath]; ok {
		return pkg, nil
	}
	meta, err := loadPackage(packagePath)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	pkg := &parsedPackage{
		meta:    meta,
		structs: map[string]structInfo{},
	}
	for _, file := range meta.GoFiles {
		path := meta.Dir + string(os.PathSeparator) + file
		parsed, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		imports := importMap(parsed)
		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				pkg.structs[typeSpec.Name.Name] = structInfo{
					file:    parsed,
					imports: imports,
					typ:     structType,
				}
			}
		}
	}
	i.packages[packagePath] = pkg
	return pkg, nil
}

func (i *inspector) structJSONFields(pkg *parsedPackage, info structInfo, stack map[string]bool) ([]sdkField, error) {
	fset := token.NewFileSet()
	var fields []sdkField
	for _, field := range info.typ.Fields.List {
		tag := jsonTag(field)
		if tag == "-" {
			continue
		}
		typeText := exprString(fset, field.Type)
		children, opaque, ref, primitive, err := i.nestedFieldsForExpr(pkg, info.imports, field.Type, stack)
		if err != nil {
			return nil, err
		}
		for _, name := range fieldNames(field) {
			if !ast.IsExported(name) {
				continue
			}
			jsonName := tag
			if jsonName == "" {
				jsonName = name
			}
			if jsonName == "-" {
				continue
			}
			fields = append(fields, sdkField{
				GoName:    name,
				JSONName:  jsonName,
				Type:      typeText,
				Fields:    children,
				Opaque:    opaque,
				TypeRef:   ref,
				Primitive: primitive,
			})
		}
	}
	return fields, nil
}

func jsonTag(field *ast.Field) string {
	if field.Tag == nil {
		return ""
	}
	value, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return ""
	}
	tag := reflect.StructTag(value).Get("json")
	if index := strings.IndexByte(tag, ','); index >= 0 {
		tag = tag[:index]
	}
	return tag
}

func fieldNames(field *ast.Field) []string {
	if len(field.Names) > 0 {
		out := make([]string, 0, len(field.Names))
		for _, name := range field.Names {
			out = append(out, name.Name)
		}
		return out
	}
	name := embeddedFieldName(field.Type)
	if name == "" {
		return nil
	}
	return []string{name}
}

func embeddedFieldName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return embeddedFieldName(v.X)
	case *ast.SelectorExpr:
		return v.Sel.Name
	default:
		return ""
	}
}

func (i *inspector) nestedFieldsForExpr(
	pkg *parsedPackage,
	imports map[string]string,
	expr ast.Expr,
	stack map[string]bool,
) ([]sdkField, bool, typeRef, bool, error) {
	ref, opaque, primitive := i.typeRefForExpr(pkg, imports, expr)
	if opaque || primitive || ref.TypeName == "" {
		return nil, opaque, ref, primitive, nil
	}
	if ref.PackagePath == "" {
		ref.PackagePath = pkg.meta.ImportPath
		ref.PackageName = pkg.meta.Name
	}
	if !isAllowedNestedPackage(ref.PackagePath, pkg.meta.ImportPath) {
		return nil, false, ref, true, nil
	}
	fields, err := i.exportedJSONFieldsFrom(ref.PackagePath, ref.TypeName, stack)
	if err != nil {
		return nil, true, ref, false, nil
	}
	if len(fields) == 0 {
		return nil, true, ref, false, nil
	}
	return fields, false, ref, false, nil
}

func (i *inspector) typeRefForExpr(pkg *parsedPackage, imports map[string]string, expr ast.Expr) (typeRef, bool, bool) {
	switch v := expr.(type) {
	case *ast.StarExpr:
		return i.typeRefForExpr(pkg, imports, v.X)
	case *ast.ArrayType:
		return i.typeRefForExpr(pkg, imports, v.Elt)
	case *ast.MapType, *ast.InterfaceType, *ast.StructType:
		return typeRef{}, true, false
	case *ast.Ident:
		if isPrimitiveType(v.Name) {
			return typeRef{}, false, true
		}
		if !ast.IsExported(v.Name) {
			return typeRef{}, false, true
		}
		if _, ok := pkg.structs[v.Name]; !ok {
			return typeRef{}, false, true
		}
		return typeRef{
			PackagePath: pkg.meta.ImportPath,
			PackageName: pkg.meta.Name,
			TypeName:    v.Name,
		}, false, false
	case *ast.SelectorExpr:
		ident, ok := v.X.(*ast.Ident)
		if !ok {
			return typeRef{}, true, false
		}
		packagePath := imports[ident.Name]
		if packagePath == "" {
			return typeRef{}, false, true
		}
		return typeRef{
			PackagePath: packagePath,
			PackageName: ident.Name,
			TypeName:    v.Sel.Name,
		}, false, false
	default:
		return typeRef{}, true, false
	}
}

func isPrimitiveType(name string) bool {
	switch name {
	case "any", "bool", "byte", "complex64", "complex128", "error", "float32", "float64",
		"int", "int8", "int16", "int32", "int64", "rune", "string",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		return true
	default:
		return false
	}
}

func isAllowedNestedPackage(packagePath, rootPackagePath string) bool {
	if packagePath == rootPackagePath {
		return true
	}
	return strings.HasPrefix(packagePath, "github.com/zscaler/zscaler-sdk-go/v3/")
}

func exprString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, expr); err != nil {
		return "<unknown>"
	}
	return buf.String()
}

func writeDraft(w io.Writer, pkg goPackage, typeName, product, resource string, fields []sdkField) {
	fmt.Fprintf(w, "# catalog draft for %s/%s from %s.%s\n", product, resource, pkg.ImportPath, typeName)
	fmt.Fprintln(w, "# Review posture: generated drafts classify every SDK field they can model.")
	fmt.Fprintln(w, "# Only approved global names render by default; ambiguous, unknown, opaque, and secret-like names stay ClassSecret.")
	fmt.Fprintln(w, "# Opaque map/interface/cyclic fields are emitted as secret so they stay dropped until explicitly modeled.")
	fmt.Fprintf(w, "# Shape-review import hint: %s %q\n", pkg.Name, pkg.ImportPath)
	fmt.Fprintln(w)
	writeCatalogDraft(w, product, resource, fields)
	fmt.Fprintln(w)
	writeShapeReviewDraft(w, pkg, typeName, product, resource, fields)
}

func writeCatalogDraft(w io.Writer, product, resource string, fields []sdkField) {
	fmt.Fprintln(w, "== internal/resources catalog seed ==")
	fmt.Fprintln(w, "{")
	fmt.Fprintf(w, "\tProduct:    %s,\n", productConst(product))
	fmt.Fprintf(w, "\tName:       %q,\n", resource)
	fmt.Fprintln(w, "\tOperations: ReadOperations(),")
	fmt.Fprintln(w, "\tFields: []FieldSpec{")
	for _, field := range fields {
		writeFieldSpec(w, field, "\t\t")
	}
	fmt.Fprintln(w, "\t},")
	fmt.Fprintln(w, "},")
}

func writeFieldSpec(w io.Writer, field sdkField, indent string) {
	classification, modes, reason := classifyField(field)
	fmt.Fprintf(w, "%s{\n", indent)
	fmt.Fprintf(w, "%s\tName:           %q,\n", indent, field.JSONName)
	fmt.Fprintf(w, "%s\tClassification: %s,\n", indent, classificationConst(classification))
	if len(modes) > 0 {
		fmt.Fprintf(w, "%s\tAllowedModes:   []redact.Mode{%s},\n", indent, formatModes(modes))
	}
	if classification == resources.ClassFreeText {
		fmt.Fprintf(w, "%s\tStandardFreeTextReason: standardFreeTextReason(%q),\n", indent, reason)
	}
	if len(field.Fields) > 0 && classification != resources.ClassSecret {
		fmt.Fprintf(w, "%s\tFields: []FieldSpec{\n", indent)
		for _, child := range field.Fields {
			writeFieldSpec(w, child, indent+"\t\t")
		}
		fmt.Fprintf(w, "%s\t},\n", indent)
	}
	fmt.Fprintf(w, "%s},\n", indent)
}

func writeShapeReviewDraft(w io.Writer, pkg goPackage, typeName, product, resource string, fields []sdkField) {
	qualifiedType := pkg.Name + "." + typeName
	fmt.Fprintln(w, "== internal/zscaler SDK shape-review seed ==")
	fmt.Fprintln(w, "{")
	fmt.Fprintf(w, "\tname:         %q,\n", qualifiedType)
	fmt.Fprintf(w, "\tresource:     resources.%s,\n", productConst(product))
	fmt.Fprintf(w, "\tresourceName: %q,\n", resource)
	fmt.Fprintf(w, "\ttyp:          reflect.TypeOf(%s{}),\n", qualifiedType)
	fmt.Fprintln(w, "\tcatalogFields: []string{")
	for _, field := range fields {
		fmt.Fprintf(w, "\t\t%q,\n", field.JSONName)
	}
	fmt.Fprintln(w, "\t},")
	fmt.Fprintln(w, "\tignoredFields: map[string]string{},")
	fmt.Fprintln(w, "},")
}

func productConst(product string) string {
	switch product {
	case "zia":
		return "ProductZIA"
	case "zpa":
		return "ProductZPA"
	default:
		return ""
	}
}

func classifyField(field sdkField) (resources.FieldClassification, []string, string) {
	normalized := normalizeFieldName(field.JSONName)
	if resources.SecretLikeFieldName(field.JSONName) || field.Opaque || ambiguousFieldName(normalized) {
		return resources.ClassSecret, nil, ""
	}
	if len(field.Fields) > 0 {
		modes := unionChildModes(field.Fields)
		if len(modes) == 0 {
			return resources.ClassSecret, nil, ""
		}
		return resources.ClassTenantConfig, modes, ""
	}
	switch {
	case isFreeTextFieldName(normalized):
		return resources.ClassFreeText, []string{"redact.ModeStandard"}, "TODO " + field.JSONName
	case isSensitiveIdentifierFieldName(normalized):
		return resources.ClassSensitiveIdentifier, []string{"redact.ModeStandard"}, ""
	case isOperationalFieldName(normalized):
		return resources.ClassOperational, []string{"redact.ModeStandard", "redact.ModeShare", "redact.ModeParanoid"}, ""
	case isTenantConfigFieldName(normalized):
		return resources.ClassTenantConfig, []string{"redact.ModeStandard", "redact.ModeShare"}, ""
	default:
		return resources.ClassSecret, nil, ""
	}
}

func unionChildModes(fields []sdkField) []string {
	seen := map[string]bool{}
	for _, field := range fields {
		classification, modes, _ := classifyField(field)
		if classification == resources.ClassSecret {
			continue
		}
		for _, mode := range modes {
			seen[mode] = true
		}
	}
	var out []string
	for _, mode := range []string{"redact.ModeStandard", "redact.ModeShare", "redact.ModeParanoid"} {
		if seen[mode] {
			out = append(out, mode)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func classificationConst(classification resources.FieldClassification) string {
	switch classification {
	case resources.ClassPublicProjectData:
		return "ClassPublicProjectData"
	case resources.ClassOperational:
		return "ClassOperational"
	case resources.ClassTenantConfig:
		return "ClassTenantConfig"
	case resources.ClassSensitiveIdentifier:
		return "ClassSensitiveIdentifier"
	case resources.ClassFreeText:
		return "ClassFreeText"
	case resources.ClassSecret:
		return "ClassSecret"
	default:
		return strconv.Quote(string(classification))
	}
}

func formatModes(modes []string) string {
	return strings.Join(modes, ", ")
}

func isFreeTextFieldName(normalized string) bool {
	switch normalized {
	case "comment", "comments", "description", "message", "notes":
		return true
	default:
		return false
	}
}

func ambiguousFieldName(normalized string) bool {
	switch normalized {
	case "attribute", "attributes", "body", "clientid", "config", "configuration",
		"content", "customerid", "data", "datum", "key", "keys", "metadata",
		"object", "objects", "parameter", "parameters", "params", "payload",
		"property", "properties", "setting", "settings", "value", "values":
		return true
	default:
		return false
	}
}

func isSensitiveIdentifierFieldName(normalized string) bool {
	sensitiveFragments := []string{
		"address",
		"city",
		"country",
		"domain",
		"email",
		"fqdn",
		"geo",
		"host",
		"ip",
		"latitude",
		"location",
		"longitude",
		"region",
		"url",
		"uri",
		"user",
	}
	for _, fragment := range sensitiveFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func isOperationalFieldName(normalized string) bool {
	if normalized == "id" {
		return true
	}
	operationalFragments := []string{
		"count",
		"deleted",
		"disabled",
		"enabled",
		"modified",
		"readonly",
		"status",
		"time",
		"type",
	}
	for _, fragment := range operationalFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func isTenantConfigFieldName(normalized string) bool {
	if normalized == "name" || strings.HasSuffix(normalized, "name") {
		return true
	}
	tenantConfigFragments := []string{
		"platform",
		"profile",
		"subcloud",
		"version",
	}
	for _, fragment := range tenantConfigFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func normalizeFieldName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
