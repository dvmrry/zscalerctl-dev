//go:build ignore

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type surface struct {
	Product        string   `json:"product"`
	Package        string   `json:"package"`
	RelativePath   string   `json:"relative_path"`
	PackageName    string   `json:"package_name"`
	Category       string   `json:"category"`
	Structs        []string `json:"structs,omitempty"`
	ReadFuncs      []string `json:"read_funcs,omitempty"`
	MutateFuncs    []string `json:"mutate_funcs,omitempty"`
	UnknownFuncs   []string `json:"unknown_funcs,omitempty"`
	AmbiguousFuncs []string `json:"ambiguous_funcs,omitempty"`
	OtherFuncs     []string `json:"other_funcs,omitempty"`
	HTTPMethods    []string `json:"http_methods,omitempty"`
	Endpoints      []string `json:"endpoints,omitempty"`
	Notes          []string `json:"notes,omitempty"`
}

type inventory struct {
	Schema     string    `json:"schema"`
	Notice     string    `json:"notice"`
	SDKModule  string    `json:"sdk_module"`
	SDKVersion string    `json:"sdk_version,omitempty"`
	SDKDir     string    `json:"sdk_dir"`
	Surfaces   []surface `json:"surfaces"`
}

type packageScan struct {
	product        string
	importPath     string
	relativePath   string
	packageName    string
	structs        set
	readFuncs      set
	mutateFuncs    set
	unknownFuncs   set
	ambiguousFuncs set
	otherFuncs     set
	httpMethods    set
	endpoints      set
	endpointHints  set
}

type set map[string]bool

func (s set) add(value string) {
	if value != "" {
		s[value] = true
	}
}

func (s set) values() []string {
	out := make([]string, 0, len(s))
	for value := range s {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

const (
	inventorySchema = "zscalerctl.sdk_surface_inventory.v1"
	inventoryNotice = "scout only: this inventory is not an enabled catalog, safety proof, entitlement check, or live response-shape validation"
)

func main() {
	var sdkDir string
	var modulePath string
	var sdkVersion string
	var format string
	flag.StringVar(&sdkDir, "sdk-dir", "vendor/github.com/zscaler/zscaler-sdk-go/v3", "Zscaler SDK module directory")
	flag.StringVar(&modulePath, "module-path", "github.com/zscaler/zscaler-sdk-go/v3", "SDK module import path")
	flag.StringVar(&sdkVersion, "sdk-version", "", "trusted SDK module version for the selected SDK tree")
	flag.StringVar(&format, "format", "markdown", "output format: markdown or json")
	flag.Parse()

	report, err := buildInventory(sdkDir, modulePath, sdkVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sdk-surface-inventory: %v\n", err)
		os.Exit(1)
	}

	switch format {
	case "markdown":
		writeMarkdown(os.Stdout, report)
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "sdk-surface-inventory: encode json: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "sdk-surface-inventory: unsupported format %q\n", format)
		os.Exit(2)
	}
}

func buildInventory(sdkDir, modulePath, sdkVersion string) (inventory, error) {
	surfaces, err := scanSDK(sdkDir, modulePath)
	if err != nil {
		return inventory{}, err
	}
	return inventory{
		Schema:     inventorySchema,
		Notice:     inventoryNotice,
		SDKModule:  modulePath,
		SDKVersion: moduleVersion(sdkDir, modulePath, sdkVersion),
		SDKDir:     filepath.Clean(sdkDir),
		Surfaces:   surfaces,
	}, nil
}

func moduleVersion(sdkDir, modulePath, explicitVersion string) string {
	if version := strings.TrimSpace(explicitVersion); version != "" {
		return version
	}

	modulesPath := selectedVendorModulesPath(sdkDir, modulePath)
	if modulesPath == "" {
		return ""
	}

	data, err := os.ReadFile(modulesPath)
	if err != nil {
		return ""
	}
	prefix := "# " + modulePath + " "
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				return fields[2]
			}
		}
	}
	return ""
}

// selectedVendorModulesPath returns modules.txt only when sdkDir is exactly
// the requested module below that same vendor directory. In particular, it
// does not fall back to the caller's working-directory vendor/modules.txt for
// an external SDK source tree.
func selectedVendorModulesPath(sdkDir, modulePath string) string {
	root, err := filepath.Abs(filepath.Clean(sdkDir))
	if err != nil {
		return ""
	}
	moduleRel := filepath.Clean(filepath.FromSlash(modulePath))
	if moduleRel == "." || moduleRel == string(filepath.Separator) {
		return ""
	}

	for dir := filepath.Dir(root); ; dir = filepath.Dir(dir) {
		if filepath.Base(dir) == "vendor" && filepath.Join(dir, moduleRel) == root {
			modulesPath := filepath.Join(dir, "modules.txt")
			info, err := os.Stat(modulesPath)
			if err == nil && !info.IsDir() {
				return modulesPath
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return ""
}

func scanSDK(sdkDir, modulePath string) ([]surface, error) {
	root := filepath.Clean(sdkDir)
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", sdkDir)
	}

	var scans []packageScan
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		switch entry.Name() {
		case ".git", "testdata":
			return filepath.SkipDir
		}
		scan, ok, err := scanPackageDir(root, modulePath, path)
		if err != nil {
			return err
		}
		if ok {
			scans = append(scans, scan)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := make([]surface, 0, len(scans))
	for _, scan := range scans {
		out = append(out, scan.toSurface())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Product != out[j].Product {
			return out[i].Product < out[j].Product
		}
		return out[i].Package < out[j].Package
	})
	return out, nil
}

func scanPackageDir(root, modulePath, dir string) (packageScan, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return packageScan{}, false, err
	}
	var goFiles []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		goFiles = append(goFiles, filepath.Join(dir, entry.Name()))
	}
	if len(goFiles) == 0 {
		return packageScan{}, false, nil
	}

	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return packageScan{}, false, err
	}
	if rel == "." {
		rel = ""
	}
	importPath := modulePath
	if rel != "" {
		importPath += "/" + filepath.ToSlash(rel)
	}

	scan := packageScan{
		product:        productForRelativePath(filepath.ToSlash(rel)),
		importPath:     importPath,
		relativePath:   filepath.ToSlash(rel),
		structs:        set{},
		readFuncs:      set{},
		mutateFuncs:    set{},
		unknownFuncs:   set{},
		ambiguousFuncs: set{},
		otherFuncs:     set{},
		httpMethods:    set{},
		endpoints:      set{},
		endpointHints:  set{},
	}

	fset := token.NewFileSet()
	for _, file := range goFiles {
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			return packageScan{}, false, err
		}
		if scan.packageName == "" {
			scan.packageName = parsed.Name.Name
		}
		scanFile(parsed, &scan)
	}
	return scan, true, nil
}

func productForRelativePath(rel string) string {
	parts := strings.Split(rel, "/")
	if len(parts) == 0 {
		return "core"
	}
	if rel == "" || parts[0] != "zscaler" {
		return "core"
	}
	if len(parts) < 2 {
		return "core"
	}
	switch parts[1] {
	case "zia", "zpa", "zcc", "zdx", "ztw", "zwa":
		return parts[1]
	case "zid":
		return "zidentity"
	default:
		return "core"
	}
}

func scanFile(file *ast.File, scan *packageScan) {
	for _, decl := range file.Decls {
		switch node := decl.(type) {
		case *ast.GenDecl:
			scanGenDecl(node, scan)
		case *ast.FuncDecl:
			scanFuncDecl(node, scan)
		}
	}
}

func scanGenDecl(decl *ast.GenDecl, scan *packageScan) {
	for _, spec := range decl.Specs {
		switch node := spec.(type) {
		case *ast.TypeSpec:
			if ast.IsExported(node.Name.Name) {
				if _, ok := node.Type.(*ast.StructType); ok {
					scan.structs.add(node.Name.Name)
				}
			}
		case *ast.ValueSpec:
			for _, value := range node.Values {
				for _, literal := range stringLiterals(value) {
					classifyLiteral(literal, scan)
				}
			}
		}
	}
}

func scanFuncDecl(decl *ast.FuncDecl, scan *packageScan) {
	methods := set{}
	endpoints := set{}
	if decl.Body != nil {
		ast.Inspect(decl.Body, func(node ast.Node) bool {
			for _, literal := range stringLiterals(node) {
				switch classifyLiteralValue(literal) {
				case "http":
					methods.add(literal)
				case "endpoint":
					endpoints.add(literal)
				}
			}
			return true
		})
	}
	for method := range methods {
		scan.httpMethods.add(method)
	}
	for endpoint := range endpoints {
		scan.endpoints.add(endpoint)
	}

	if !ast.IsExported(decl.Name.Name) {
		return
	}
	name := decl.Name.Name
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		if recv := receiverName(decl.Recv.List[0].Type); recv != "" {
			name = recv + "." + name
		}
	}

	classification := classifyFunction(decl.Name.Name, methods)
	for _, reason := range classification.ambiguous {
		scan.ambiguousFuncs.add(name + ": " + reason)
	}
	switch classification.kind {
	case "read":
		scan.readFuncs.add(name)
	case "mutate":
		scan.mutateFuncs.add(name)
	case "unknown":
		scan.unknownFuncs.add(name)
	default:
		scan.otherFuncs.add(name)
	}
}

func receiverName(expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.StarExpr:
		return receiverName(node.X)
	case *ast.IndexExpr:
		return receiverName(node.X)
	case *ast.IndexListExpr:
		return receiverName(node.X)
	default:
		return ""
	}
}

func stringLiterals(node ast.Node) []string {
	lit, ok := node.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return nil
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return nil
	}
	return []string{value}
}

func classifyLiteral(value string, scan *packageScan) {
	switch classifyLiteralValue(value) {
	case "http":
		scan.httpMethods.add(value)
	case "endpoint":
		scan.endpoints.add(value)
	}
}

func classifyLiteralValue(value string) string {
	switch value {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return "http"
	}
	if strings.HasPrefix(value, "/") && strings.Contains(value, "api") {
		return "endpoint"
	}
	if strings.HasPrefix(value, "/admin/") || strings.HasPrefix(value, "/zcc") || strings.HasPrefix(value, "/zdx") || strings.HasPrefix(value, "/ztw") || strings.HasPrefix(value, "/zpa") || strings.HasPrefix(value, "/zia") || strings.HasPrefix(value, "/zscsb") {
		return "endpoint"
	}
	return ""
}

type functionClassification struct {
	kind      string
	ambiguous []string
}

func classifyFunction(name string, methods set) functionClassification {
	nameKind := classifyFunctionName(name)
	methodKind := classifyHTTPMethods(methods)

	switch methodKind {
	case "mixed":
		return functionClassification{kind: "mutate", ambiguous: []string{"mixed read and mutating HTTP methods"}}
	case "mutate":
		classification := functionClassification{kind: "mutate"}
		if nameKind == "read" {
			classification.ambiguous = append(classification.ambiguous, "read-like name with mutating HTTP method")
		}
		return classification
	case "read":
		switch nameKind {
		case "mutate":
			return functionClassification{kind: "mutate", ambiguous: []string{"mutating-like name with read HTTP method"}}
		case "unknown":
			return functionClassification{kind: "read", ambiguous: []string{"unknown name with read HTTP method"}}
		default:
			return functionClassification{kind: "read"}
		}
	default:
		switch nameKind {
		case "read", "mutate":
			return functionClassification{kind: nameKind}
		default:
			return functionClassification{kind: "unknown"}
		}
	}
}

func classifyFunctionName(name string) string {
	if hasMutatingPrefix(name) {
		return "mutate"
	}
	if hasReadPrefix(name) {
		return "read"
	}
	return "unknown"
}

func classifyHTTPMethods(methods set) string {
	hasRead := hasReadMethod(methods)
	hasMutate := hasMutatingMethod(methods)
	switch {
	case hasRead && hasMutate:
		return "mixed"
	case hasMutate:
		return "mutate"
	case hasRead:
		return "read"
	default:
		return "unknown"
	}
}

func hasReadMethod(methods set) bool {
	return len(methods) > 0 && methods["GET"]
}

func hasMutatingMethod(methods set) bool {
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		if methods[method] {
			return true
		}
	}
	return false
}

func hasReadPrefix(name string) bool {
	for _, prefix := range []string{"Get", "List", "Read", "Search", "Lookup", "Download"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func hasMutatingPrefix(name string) bool {
	for _, prefix := range []string{
		"Activate", "Add", "Apply", "Approve", "Assign", "Bulk", "Cancel",
		"Clone", "Create", "Delete", "Disable", "Enable", "Generate",
		"Import", "Invalidate", "Move", "Patch", "Provision", "Refresh",
		"Register", "Remove", "Renew", "Reorder", "Resume", "Revoke",
		"Rotate", "Set", "Submit", "Suspend", "Sync", "Trigger",
		"Unassign", "Update", "Upload",
	} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func (scan packageScan) toSurface() surface {
	endpoints := scan.endpoints.values()
	product := scan.product
	if product == "core" && hasAdminEndpoint(endpoints) {
		product = "zidentity"
	}
	out := surface{
		Product:        product,
		Package:        scan.importPath,
		RelativePath:   scan.relativePath,
		PackageName:    scan.packageName,
		Structs:        scan.structs.values(),
		ReadFuncs:      scan.readFuncs.values(),
		MutateFuncs:    scan.mutateFuncs.values(),
		UnknownFuncs:   scan.unknownFuncs.values(),
		AmbiguousFuncs: scan.ambiguousFuncs.values(),
		OtherFuncs:     scan.otherFuncs.values(),
		HTTPMethods:    scan.httpMethods.values(),
		Endpoints:      endpoints,
	}
	out.Category = categoryFor(out)
	out.Notes = notesFor(out)
	return out
}

func hasAdminEndpoint(endpoints []string) bool {
	for _, endpoint := range endpoints {
		if strings.HasPrefix(endpoint, "/admin/api/") {
			return true
		}
	}
	return false
}

func categoryFor(item surface) string {
	if isProductRootPackage(item) {
		return "product-client-config"
	}
	hasRead := len(item.ReadFuncs) > 0
	hasMutate := len(item.MutateFuncs) > 0
	hasList := hasListFunction(item.ReadFuncs)
	hasGet := hasGetFunction(item.ReadFuncs)
	switch {
	case hasRead && hasList && hasGet && hasMutate:
		return "list-get-with-mutating-neighbors"
	case hasRead && hasList && hasGet:
		return "ordinary-list-get"
	case hasRead && hasMutate:
		return "mixed-read-write-sdk-package"
	case hasRead:
		return "read-only-nonstandard"
	case hasMutate:
		return "mutating-only"
	case len(item.Structs) > 0:
		return "types-or-client-config"
	default:
		return "other"
	}
}

func isProductRootPackage(item surface) bool {
	switch item.RelativePath {
	case "zscaler/zia", "zscaler/zpa", "zscaler/zcc", "zscaler/zdx", "zscaler/zid", "zscaler/ztw", "zscaler/zwa":
		return true
	default:
		return false
	}
}

func hasListFunction(funcs []string) bool {
	for _, name := range funcs {
		base := baseFunctionName(name)
		if strings.HasPrefix(base, "List") || strings.Contains(base, "GetAll") || strings.Contains(base, "GetLite") {
			return true
		}
	}
	return false
}

func hasGetFunction(funcs []string) bool {
	for _, name := range funcs {
		base := baseFunctionName(name)
		if strings.HasPrefix(base, "Get") && !strings.Contains(base, "GetAll") && !strings.Contains(base, "GetLite") {
			return true
		}
	}
	return false
}

func baseFunctionName(name string) string {
	if index := strings.LastIndexByte(name, '.'); index >= 0 {
		return name[index+1:]
	}
	return name
}

func notesFor(item surface) []string {
	var notes []string
	if len(item.AmbiguousFuncs) > 0 {
		notes = append(notes, "ambiguous function signals detected; manual review required before queueing")
	}
	if len(item.UnknownFuncs) > 0 {
		notes = append(notes, "exported funcs with unknown verb detected; treat as shape-decision work")
	}
	if item.Product == "zidentity" {
		notes = append(notes, "admin/zidentity URL routing appears in core client code; treat as identity-plane work")
	}
	if item.Category == "product-client-config" && item.Product != "zia" {
		notes = append(notes, "product client/config package detected; no high-level resource service package detected in this SDK snapshot")
	}
	if item.Category == "list-get-with-mutating-neighbors" || item.Category == "mixed-read-write-sdk-package" {
		notes = append(notes, "package contains mutating SDK helpers; zscalerctl must wire only read funcs")
	}
	if item.Product != "core" && item.Product != "zia" && item.Product != "zidentity" && item.Category == "types-or-client-config" {
		notes = append(notes, "no high-level resource service package detected in this SDK snapshot")
	}
	if len(item.Endpoints) == 0 && len(item.ReadFuncs) > 0 {
		notes = append(notes, "read function detected without static endpoint literal")
	}
	return notes
}

func writeMarkdown(out *os.File, report inventory) {
	fmt.Fprintln(out, "# SDK Surface Inventory")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Generated from the vendored Zscaler SDK. This is a scouting report, not an enabled resource catalog.")
	fmt.Fprintf(out, "Notice: %s.\n", report.Notice)
	if report.SDKVersion != "" {
		fmt.Fprintf(out, "SDK module: `%s` `%s` from `%s`.\n", report.SDKModule, report.SDKVersion, report.SDKDir)
	} else {
		fmt.Fprintf(out, "SDK module: `%s` from `%s`.\n", report.SDKModule, report.SDKDir)
	}
	fmt.Fprintln(out)
	writeSummary(out, report.Surfaces)
	fmt.Fprintln(out)
	writeSurfaceTable(out, report.Surfaces)
}

func writeSummary(out *os.File, surfaces []surface) {
	type counts struct {
		packages  int
		read      int
		listGet   int
		unknown   int
		ambiguous int
	}
	byProduct := map[string]*counts{}
	for _, item := range surfaces {
		if byProduct[item.Product] == nil {
			byProduct[item.Product] = &counts{}
		}
		byProduct[item.Product].packages++
		if len(item.ReadFuncs) > 0 {
			byProduct[item.Product].read++
		}
		if item.Category == "ordinary-list-get" || item.Category == "list-get-with-mutating-neighbors" {
			byProduct[item.Product].listGet++
		}
		if len(item.UnknownFuncs) > 0 {
			byProduct[item.Product].unknown++
		}
		if len(item.AmbiguousFuncs) > 0 {
			byProduct[item.Product].ambiguous++
		}
	}
	var products []string
	for product := range byProduct {
		products = append(products, product)
	}
	sort.Strings(products)
	fmt.Fprintln(out, "## Summary")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "| Product | Packages | Packages with read funcs | List/get-shaped packages | Packages with unknown funcs | Packages with ambiguous funcs |")
	fmt.Fprintln(out, "| --- | ---: | ---: | ---: | ---: | ---: |")
	for _, product := range products {
		count := byProduct[product]
		fmt.Fprintf(out, "| `%s` | %d | %d | %d | %d | %d |\n", product, count.packages, count.read, count.listGet, count.unknown, count.ambiguous)
	}
}

func writeSurfaceTable(out *os.File, surfaces []surface) {
	fmt.Fprintln(out, "## Surfaces")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "| Product | Package | Category | Read funcs | Mutating funcs | Unknown funcs | Ambiguous funcs | Endpoints | Notes |")
	fmt.Fprintln(out, "| --- | --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, item := range surfaces {
		if item.Category == "other" && len(item.Endpoints) == 0 && len(item.UnknownFuncs) == 0 && len(item.AmbiguousFuncs) == 0 {
			continue
		}
		fmt.Fprintf(
			out,
			"| `%s` | `%s` | `%s` | %s | %s | %s | %s | %s | %s |\n",
			item.Product,
			escapeTable(item.RelativePath),
			item.Category,
			inlineList(item.ReadFuncs, 8),
			inlineList(item.MutateFuncs, 8),
			inlineList(item.UnknownFuncs, 6),
			inlineList(item.AmbiguousFuncs, 6),
			inlineList(item.Endpoints, 4),
			inlineList(item.Notes, 3),
		)
	}
}

func inlineList(values []string, limit int) string {
	if len(values) == 0 {
		return ""
	}
	truncated := values
	suffix := ""
	if len(values) > limit {
		truncated = values[:limit]
		suffix = fmt.Sprintf("<br>+%d more", len(values)-limit)
	}
	escaped := make([]string, 0, len(truncated))
	for _, value := range truncated {
		escaped = append(escaped, "`"+escapeTable(value)+"`")
	}
	return strings.Join(escaped, "<br>") + suffix
}

func escapeTable(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
