package cli

// introspect.go — shared walk that produces IntrospectDoc from the live Cobra
// tree, the resource catalog, and the global-flag definitions.
//
// IntrospectTree is the single exported entry point. It is consumed by:
//   - internal/cli (introspect command, Task 1.2): calls IntrospectTree, sets
//     CLIVersion, and serialises to JSON.
//
// The tree enumeration is driven by WalkCobraTree (see its doc comment), which
// is also used directly by scripts/gen-cli-docs.go (markdown generator).
// Walking from the same function ensures docs and the agent JSON map always
// enumerate the identical command set and cannot drift.
//
// No-leak contract: IntrospectTree emits ONLY static structure (command/flag
// names, descriptions, catalog names, exit-code text). It must remain
// config-free: it must NOT call LoadConfig, construct a reader, or touch the
// network. BuildCommandTree already uses zero-value globalOptions and is
// config-free.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// schemaURL is the canonical JSON Schema URL for IntrospectDoc. The file
// docs/schema/introspect.schema.json is published separately and the URL
// intentionally floats on main; the introspect_version field carries the real
// contract version.
const schemaURL = "https://raw.githubusercontent.com/dvmrry/zscalerctl/main/docs/schema/introspect.schema.json"

// IntrospectDoc is the top-level document returned by IntrospectTree.
type IntrospectDoc struct {
	Schema            string        `json:"$schema"`
	IntrospectVersion string        `json:"introspect_version"`
	CLIVersion        string        `json:"cli_version"`
	ReadOnly          bool          `json:"read_only"`
	GlobalFlags       []FlagDoc     `json:"global_flags"`
	Commands          []CommandDoc  `json:"commands"`
	Catalog           CatalogDoc    `json:"catalog"`
	ExitCodes         []ExitCodeDoc `json:"exit_codes"`
}

// OutputSafe implements output.SafeJSON. IntrospectDoc emits only static
// structure (command names, descriptions, catalog names, exit-code text) — no
// tenant data, credentials, or runtime values.
func (IntrospectDoc) OutputSafe() {}

// CommandDoc describes a single CLI command for agent consumption.
type CommandDoc struct {
	Path           string    `json:"path"`
	Short          string    `json:"short"`
	Long           string    `json:"long,omitempty"`
	Aliases        []string  `json:"aliases"`
	Hidden         bool      `json:"hidden"`
	Deprecated     string    `json:"deprecated,omitempty"`
	Mutating       bool      `json:"mutating"`
	Args           ArgsDoc   `json:"args"`
	Flags          []FlagDoc `json:"flags"`
	InheritedFlags []string  `json:"inherited_flags"`
	OutputFields   []string  `json:"output_fields,omitempty"`
}

// FlagDoc describes a single flag (global or local).
type FlagDoc struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Default   string `json:"default,omitempty"`
	// Required is never true: the project deliberately avoids required flags.
	// The field is retained for forward-compatibility with the published schema.
	Required bool     `json:"required,omitempty"`
	Usage    string   `json:"usage"`
	Enum     []string `json:"enum,omitempty"`
}

// ArgsDoc describes the positional argument policy for a command.
type ArgsDoc struct {
	Policy      string   `json:"policy"`
	N           int      `json:"n,omitempty"`
	ValidValues []string `json:"valid_values,omitempty"`
}

// CatalogDoc is the catalog section of IntrospectDoc.
type CatalogDoc struct {
	Products  []string      `json:"products"`
	Resources []ResourceDoc `json:"resources"`
}

// ResourceDoc describes a single catalog resource entry.
type ResourceDoc struct {
	Product string   `json:"product"`
	Name    string   `json:"name"`
	Ops     []string `json:"ops"`
	Fields  []string `json:"fields"`
}

// ExitCodeDoc describes a single exit-code mapping.
//
// SOURCE OF TRUTH: the exit-code constants and exitCodeForError in
// cmd/zscalerctl/main.go. Keep this table in sync with that file.
type ExitCodeDoc struct {
	Code        int    `json:"code"`
	Kind        string `json:"kind"`
	Retryable   bool   `json:"retryable"`
	Description string `json:"description"`
}

// globalFlagEnums maps global flag names to their allowed enumeration values.
// These mirror the completion values wired in registerGlobalFlagCompletions.
var globalFlagEnums = map[string][]string{
	"format":    completionFormats,
	"redaction": completionRedaction,
	"color":     completionColors,
	"log-level": completionLogLevels,
}

// IntrospectTree walks the live Cobra command tree and the resource catalog to
// produce a fully-populated IntrospectDoc. It is config-free: no credentials,
// config files, or network calls are made.
//
// The real Cobra commands are enumerated by calling WalkCobraTree — the same
// shared depth-first walk used by scripts/gen-cli-docs.go for markdown
// generation. Virtual "{product} {resource} {op}" entries (e.g. "zia locations
// list") are sourced separately from resources.Catalog(), which is already the
// single source of truth for resources.
//
// CLIVersion is intentionally left empty. The caller (e.g. newIntrospectCmd in
// Task 1.2) is responsible for setting it from version.Current().
func IntrospectTree(a *App) IntrospectDoc {
	root := BuildCommandTree(a)
	// InitDefaultCompletionCmd mirrors what gen-cli-docs does so the completion
	// subcommand is included in both the docs and the introspect output.
	root.InitDefaultCompletionCmd()

	doc := IntrospectDoc{
		Schema:            schemaURL,
		IntrospectVersion: "1",
		CLIVersion:        "",
		ReadOnly:          true,
		GlobalFlags:       buildGlobalFlags(),
		Catalog:           buildCatalog(),
		ExitCodes:         buildExitCodes(),
	}

	// WalkCobraTree drives the real-command enumeration in depth-first,
	// alphabetically-sorted order (identical to gen-cli-docs). For product
	// group nodes (zia, zpa, etc.) we skip emitting a bare product CommandDoc
	// and instead append virtual {product} {resource} {op} entries from the
	// catalog. All other commands (including hidden/deprecated) are included so
	// agents see the full command surface.
	var cmds []CommandDoc
	WalkCobraTree(root, func(cmd *cobra.Command, path string) {
		// A command is a product group node if it is a direct child of root
		// (no space in path) and its name is a known product in the catalog.
		if !strings.Contains(path, " ") && knownProductCommand(cmd.Name()) {
			// Do not emit a CommandDoc for the bare "zia" / "zpa" / … node.
			// Synthesize virtual entries for each {product} {resource} {op}
			// triple from the catalog instead.
			cmds = append(cmds, buildProductResourceDocs(cmd, path)...)
			return
		}
		cmds = append(cmds, buildSingleCommandDoc(cmd, path))
	})

	doc.Commands = cmds
	return doc
}

// buildGlobalFlags constructs FlagDoc entries from the canonical globalFlagDefs.
// This is the single source of truth; no raw pflag scanning is needed here.
func buildGlobalFlags() []FlagDoc {
	docs := make([]FlagDoc, 0, len(globalFlagDefs))
	for _, d := range globalFlagDefs {
		def := d.defaultVal
		if d.kind == "stringArray" && def == "[]" {
			def = ""
		}
		fd := FlagDoc{
			Name:    d.name,
			Type:    d.kind,
			Default: def,
			Usage:   d.usage,
			Enum:    globalFlagEnums[d.name],
		}
		docs = append(docs, fd)
	}
	return docs
}

// buildCatalog converts resources.Catalog() into a CatalogDoc. Fields are the
// standard-mode projected field names (what an agent can use with --fields /
// --filter). Secret and standard-excluded fields are omitted.
func buildCatalog() CatalogDoc {
	cat := resources.Catalog()

	// Collect ordered, deduplicated product list.
	seen := make(map[resources.Product]bool)
	products := make([]string, 0)
	for _, spec := range cat {
		if !seen[spec.Product] {
			seen[spec.Product] = true
			products = append(products, string(spec.Product))
		}
	}

	resDocs := make([]ResourceDoc, 0, len(cat))
	for _, spec := range cat {
		ops := readOperationNames(spec)
		fields := spec.FieldOrder(redact.ModeStandard)
		resDocs = append(resDocs, ResourceDoc{
			Product: string(spec.Product),
			Name:    spec.Name,
			Ops:     ops,
			Fields:  fields,
		})
	}

	return CatalogDoc{
		Products:  products,
		Resources: resDocs,
	}
}

// buildExitCodes returns the static exit-code table.
//
// SOURCE OF TRUTH: cmd/zscalerctl/main.go (exitCodeForError, constants block).
// Update this table whenever exit codes change in that file.
//
// Note: ExitCodeDoc.Kind is the coarse exit-code category, NOT necessarily
// identical to the per-error JSON envelope "kind" field produced by errorKind.
// For example, exit code 4 covers both the "not_found" and "unsupported_resource"
// JSON error kinds because exitCodeForError maps both to exitNotFound.
func buildExitCodes() []ExitCodeDoc {
	return []ExitCodeDoc{
		{Code: 0, Kind: "ok", Retryable: false,
			Description: "success"},
		{Code: 1, Kind: "internal", Retryable: false,
			Description: "unexpected internal error (bug or panic)"},
		{Code: 2, Kind: "usage", Retryable: false,
			Description: "invalid command syntax, flag, argument, resource ID, proxy config, or config file (JSON error kind may be usage, invalid_resource_id, invalid_proxy_config, or invalid_config)"},
		{Code: 3, Kind: "missing_credentials", Retryable: false,
			Description: "credentials not configured or incomplete"},
		{Code: 4, Kind: "not_found", Retryable: false,
			Description: "resource, operation, or ID not found (JSON error kind may be not_found or unsupported_resource)"},
		{Code: 5, Kind: "live_access_failed", Retryable: true,
			Description: "Zscaler API call failed (network, auth token, or quota); transient — retry is reasonable"},
		{Code: 6, Kind: "partial_dump", Retryable: false,
			Description: "dump completed but some resources failed; see errors.ndjson in the output directory"},
		{Code: 7, Kind: "drift_detected", Retryable: false,
			Description: "drift check found differences between two snapshots"},
	}
}

// buildSingleCommandDoc builds one CommandDoc for the given cobra command.
// path is the space-joined command path without the root program name
// (e.g. "config", "config init", "zia url-lookup"). Product group nodes
// (zia, zpa, etc.) are handled separately by the caller; this function is
// only called for real leaf/branch commands that should appear as-is.
func buildSingleCommandDoc(cmd *cobra.Command, path string) CommandDoc {
	// Fix #3: completion subcommand Long prose triggers the redactor (it
	// contains "source <(..." which matches the secret-assignment pattern).
	// The shell-setup instructions are not useful to agents; suppress Long for
	// any command whose path starts with "completion".
	longText := strings.TrimSpace(cmd.Long)
	if strings.HasPrefix(path, "completion") {
		longText = ""
	}

	// Fix #8: populate OutputFields for commands that annotate their fields.
	var outputFields []string
	if ann, ok := cmd.Annotations["introspect/output-fields"]; ok && ann != "" {
		outputFields = strings.Split(ann, ",")
	}

	doc := CommandDoc{
		Path:           path,
		Short:          cmd.Short,
		Long:           longText,
		Aliases:        cmd.Aliases,
		Hidden:         cmd.Hidden,
		Deprecated:     cmd.Deprecated,
		Mutating:       cmd.Annotations["introspect/mutating"] == "true",
		Args:           buildArgsDoc(cmd),
		Flags:          buildLocalFlagDocs(cmd),
		InheritedFlags: buildInheritedFlagNames(cmd),
		OutputFields:   outputFields,
	}
	if doc.Aliases == nil {
		doc.Aliases = []string{}
	}
	return doc
}

// buildProductResourceDocs synthesizes CommandDoc entries for each
// {product} {resource} {op} triple in the catalog. The product Cobra command
// provides inherited flag context (the globals); each virtual entry gets the
// inherited flag names from it.
func buildProductResourceDocs(productCmd *cobra.Command, productPath string) []CommandDoc {
	product := resources.Product(productCmd.Name())
	cat := resources.Catalog()

	inheritedNames := buildInheritedFlagNames(productCmd)

	var docs []CommandDoc
	for _, spec := range cat {
		if spec.Product != product {
			continue
		}
		fields := spec.FieldOrder(redact.ModeStandard)
		for _, op := range spec.Operations {
			if op.Capability != resources.CapabilityRead {
				continue
			}
			path := productPath + " " + spec.Name + " " + op.Name
			var argsPolicy ArgsDoc
			switch op.Name {
			case "get":
				argsPolicy = ArgsDoc{Policy: "exact", N: 1, ValidValues: nil}
			case "show":
				argsPolicy = ArgsDoc{Policy: "none"}
			default: // list
				argsPolicy = ArgsDoc{Policy: "none"}
			}
			doc := CommandDoc{
				Path:           path,
				Short:          op.Name + " " + string(product) + " " + spec.Name,
				Long:           "",
				Aliases:        []string{},
				Hidden:         false,
				Deprecated:     "",
				Mutating:       false,
				Args:           argsPolicy,
				Flags:          []FlagDoc{},
				InheritedFlags: inheritedNames,
				OutputFields:   fields,
			}
			docs = append(docs, doc)
		}
	}
	return docs
}

// buildArgsDoc derives an ArgsDoc from the command's ValidArgs, the
// "introspect/args-policy" annotation, and the Use string (in that priority).
//
// Annotation format: "exact:N" or "at_least:N" (e.g. "exact:2", "at_least:1").
// When the annotation is present it overrides the Use-suffix heuristic entirely.
// The heuristic remains the fallback for commands with no annotation.
func buildArgsDoc(cmd *cobra.Command) ArgsDoc {
	if len(cmd.ValidArgs) > 0 {
		// ValidArgs is the set of valid values for ONE positional argument, so
		// N=1 is correct even when len(ValidArgs) > 1 — do not "fix" this.
		return ArgsDoc{
			Policy:      "exact",
			N:           1,
			ValidValues: cmd.ValidArgs,
		}
	}
	// Explicit annotation overrides the Use-suffix heuristic.
	if ann, ok := cmd.Annotations["introspect/args-policy"]; ok {
		if doc, parsed := parseArgsPolicyAnnotation(ann); parsed {
			return doc
		}
	}
	// Derive from Use suffix — look for common arg patterns.
	suffix := usageSuffixIntrospect(cmd)
	switch {
	case suffix == "" || suffix == "[flags]":
		return ArgsDoc{Policy: "none"}
	case strings.HasPrefix(suffix, "[") && strings.HasSuffix(suffix, "]"):
		// Optional args like "[id]"
		return ArgsDoc{Policy: "range", N: 1}
	default:
		return ArgsDoc{Policy: "arbitrary"}
	}
}

// parseArgsPolicyAnnotation parses an "introspect/args-policy" annotation value
// of the form "exact:N" or "at_least:N" into an ArgsDoc. Returns (doc, true) on
// success, (ArgsDoc{}, false) if the annotation is not in a recognised format.
func parseArgsPolicyAnnotation(ann string) (ArgsDoc, bool) {
	idx := strings.LastIndex(ann, ":")
	if idx < 0 {
		return ArgsDoc{}, false
	}
	policy := ann[:idx]
	var n int
	if _, err := fmt.Sscanf(ann[idx+1:], "%d", &n); err != nil {
		return ArgsDoc{}, false
	}
	switch policy {
	case "exact", "at_least":
		return ArgsDoc{Policy: policy, N: n}, true
	}
	return ArgsDoc{}, false
}

// usageSuffixIntrospect extracts the argument portion from cmd.Use (everything
// after the first word). Mirrors usageSuffix in gen-cli-docs.go but is kept
// local to avoid a dependency on the //go:build ignore script.
func usageSuffixIntrospect(cmd *cobra.Command) string {
	use := strings.TrimSpace(cmd.Use)
	idx := strings.Index(use, " ")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(use[idx:])
}

// buildLocalFlagDocs converts cmd.NonInheritedFlags() to a []FlagDoc. Only
// local (non-inherited) flags are included here; inherited flag names are
// captured separately by buildInheritedFlagNames.
func buildLocalFlagDocs(cmd *cobra.Command) []FlagDoc {
	docs := make([]FlagDoc, 0)
	cmd.NonInheritedFlags().VisitAll(func(f *pflag.Flag) {
		def := f.DefValue
		if f.Value.Type() == "stringArray" && def == "[]" {
			def = ""
		}
		fd := FlagDoc{
			Name:      f.Name,
			Shorthand: f.Shorthand,
			Type:      f.Value.Type(),
			Default:   def,
			Usage:     f.Usage,
		}
		docs = append(docs, fd)
	})
	return docs
}

// buildInheritedFlagNames returns the names of flags inherited from parent
// commands (i.e. cmd.InheritedFlags()), without the leading "--".
func buildInheritedFlagNames(cmd *cobra.Command) []string {
	names := make([]string, 0)
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		names = append(names, f.Name)
	})
	return names
}

// WalkCobraTree visits each non-root command in the Cobra tree rooted at root,
// calling fn(cmd, path) for each one in depth-first, alphabetically-sorted
// order. path is the space-joined command path without the root program name
// (e.g. "config", "config init").
//
// This is the genuine single shared walk over the real Cobra command tree. It
// is used by:
//   - scripts/gen-cli-docs.go (markdown generator): drives the per-command
//     section output; render-time filtering (e.g. cmd.Hidden) is the caller's
//     responsibility.
//   - IntrospectTree (JSON agent map): drives the real-command entries. Virtual
//     "{product} {resource} {op}" entries (e.g. "zia locations list") are NOT
//     produced here; they are sourced separately from resources.Catalog() by
//     IntrospectTree.
//
// Walking from the same function ensures that the generated markdown and the
// agent JSON map always enumerate the identical real Cobra command set and
// cannot drift from one another.
func WalkCobraTree(root *cobra.Command, fn func(cmd *cobra.Command, path string)) {
	walkCobraSubtree(root, "", fn)
}

func walkCobraSubtree(cmd *cobra.Command, parentPath string, fn func(*cobra.Command, string)) {
	subs := cmd.Commands()
	// Sort alphabetically — matches gen-cli-docs sort order (deterministic).
	sortedSubs := make([]*cobra.Command, len(subs))
	copy(sortedSubs, subs)
	sort.Slice(sortedSubs, func(i, j int) bool {
		return sortedSubs[i].Name() < sortedSubs[j].Name()
	})
	for _, sub := range sortedSubs {
		var path string
		if parentPath == "" {
			path = sub.Name()
		} else {
			path = parentPath + " " + sub.Name()
		}
		fn(sub, path)
		walkCobraSubtree(sub, path, fn)
	}
}

// newIntrospectCmd returns the Cobra "introspect" subcommand. It is config-free:
// it does NOT call LoadConfig, build a reader, or touch the network. The output
// is the static CLI surface map (commands, flags, catalog, exit codes) as JSON
// by default, or as a human-readable indented tree with --format table/pretty.
//
// FormatAuto resolves to JSON here (machine-first by default). Only explicit
// --format table/--format pretty produces the human tree renderer.
// --format ndjson is rejected: introspect is a single document, not a stream.
func (a *App) newIntrospectCmd(opts globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "introspect",
		Short: "print a machine-readable map of all commands, flags, and resources (JSON)",
		RunE: func(_ *cobra.Command, args []string) error {
			return a.runIntrospect(opts, args)
		},
	}
}

func (a *App) runIntrospect(opts globalOptions, args []string) error {
	if err := requireNoArgs("introspect", args); err != nil {
		return err
	}
	doc := IntrospectTree(a)
	doc.CLIVersion = version.Current().Version
	// JSON is the happy-path (machine-first default); auto resolves to JSON for
	// non-TTY and to pretty for TTY via resolveFormat before RunE fires.
	if opts.format == output.FormatJSON {
		return output.NewRenderer(redact.New(redact.ModeStandard)).WriteJSON(a.out, doc)
	}
	if opts.format == output.FormatTable || opts.format == output.FormatPretty {
		treeText := introspectTreeText(doc)
		return output.NewRenderer(redact.New(redact.ModeStandard)).WriteText(a.out, output.NewSafeText(treeText))
	}
	// ndjson and any future unrecognised formats are rejected.
	return rejectUnsupportedFormat("introspect", opts.format)
}

// introspectTreeText renders doc as a human-readable indented text tree.
// It is a secondary convenience for humans; the primary output is JSON.
// Content is static structure only (no tenant data).
func introspectTreeText(doc IntrospectDoc) string {
	var b strings.Builder

	fmt.Fprintf(&b, "zscalerctl CLI surface map\n")
	fmt.Fprintf(&b, "  version:   %s\n", doc.CLIVersion)
	fmt.Fprintf(&b, "  read-only: %v\n", doc.ReadOnly)
	fmt.Fprintf(&b, "  schema:    %s\n\n", doc.Schema)

	// Global flags.
	fmt.Fprintf(&b, "Global flags (%d):\n", len(doc.GlobalFlags))
	for _, f := range doc.GlobalFlags {
		if f.Default != "" {
			fmt.Fprintf(&b, "  --%s (%s, default %q)  %s\n", f.Name, f.Type, f.Default, f.Usage)
		} else {
			fmt.Fprintf(&b, "  --%s (%s)  %s\n", f.Name, f.Type, f.Usage)
		}
	}
	b.WriteString("\n")

	// Commands.
	fmt.Fprintf(&b, "Commands (%d):\n", len(doc.Commands))
	for _, cmd := range doc.Commands {
		hidden := ""
		if cmd.Hidden {
			hidden = " [hidden]"
		}
		fmt.Fprintf(&b, "  %-40s  %s%s\n", cmd.Path, cmd.Short, hidden)
		for _, f := range cmd.Flags {
			if f.Default != "" {
				fmt.Fprintf(&b, "      --%s (%s, default %q)\n", f.Name, f.Type, f.Default)
			} else {
				fmt.Fprintf(&b, "      --%s (%s)\n", f.Name, f.Type)
			}
		}
	}
	b.WriteString("\n")

	// Catalog summary.
	fmt.Fprintf(&b, "Catalog: %d products, %d resources\n",
		len(doc.Catalog.Products), len(doc.Catalog.Resources))
	resourcesPerProduct := make(map[string]int, len(doc.Catalog.Products))
	for _, r := range doc.Catalog.Resources {
		resourcesPerProduct[r.Product]++
	}
	for _, p := range doc.Catalog.Products {
		fmt.Fprintf(&b, "  %s: %d resource(s)\n", p, resourcesPerProduct[p])
	}
	b.WriteString("\n")

	// Exit codes.
	fmt.Fprintf(&b, "Exit codes (%d):\n", len(doc.ExitCodes))
	for _, ec := range doc.ExitCodes {
		retryable := ""
		if ec.Retryable {
			retryable = " [retryable]"
		}
		fmt.Fprintf(&b, "  %d  %-25s  %s%s\n", ec.Code, ec.Kind, ec.Description, retryable)
	}

	return b.String()
}
