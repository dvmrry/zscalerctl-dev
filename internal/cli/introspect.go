package cli

// introspect.go — shared walk that produces IntrospectDoc from the live Cobra
// tree, the resource catalog, and the global-flag definitions.
//
// IntrospectTree is the single exported entry point. It is consumed by:
//   - scripts/gen-cli-docs.go (markdown generator): iterates IntrospectDoc.Commands
//     instead of re-walking the raw *cobra.Command tree, ensuring docs and the
//     agent JSON map cannot drift from one another.
//   - internal/cli (introspect command, Task 1.2): calls IntrospectTree, sets
//     CLIVersion, and serialises to JSON.
//
// No-leak contract: IntrospectTree emits ONLY static structure (command/flag
// names, descriptions, catalog names, exit-code text). It must remain
// config-free: it must NOT call LoadConfig, construct a reader, or touch the
// network. BuildCommandTree already uses zero-value globalOptions and is
// config-free.

import (
	"strings"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// schemaURL is the canonical JSON Schema URL for IntrospectDoc.
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
	Name      string   `json:"name"`
	Shorthand string   `json:"shorthand,omitempty"`
	Type      string   `json:"type"`
	Default   string   `json:"default,omitempty"`
	Required  bool     `json:"required,omitempty"`
	Usage     string   `json:"usage"`
	Enum      []string `json:"enum,omitempty"`
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

	// Walk the full command tree, stripping the root program name from paths.
	doc.Commands = walkCommandTree(root, "")

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
	var products []string
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
func buildExitCodes() []ExitCodeDoc {
	return []ExitCodeDoc{
		{Code: 0, Kind: "ok", Retryable: false,
			Description: "success"},
		{Code: 1, Kind: "internal", Retryable: false,
			Description: "unexpected internal error (bug or panic)"},
		{Code: 2, Kind: "usage", Retryable: false,
			Description: "invalid command syntax, flag, or argument"},
		{Code: 3, Kind: "missing_credentials", Retryable: false,
			Description: "credentials not configured or incomplete"},
		{Code: 4, Kind: "not_found", Retryable: false,
			Description: "requested resource or operation is not found"},
		{Code: 5, Kind: "live_access_failed", Retryable: true,
			Description: "Zscaler API call failed (network, auth token, or quota); transient — retry is reasonable"},
		{Code: 6, Kind: "partial_dump", Retryable: false,
			Description: "dump completed but some resources failed; see errors.ndjson in the output directory"},
		{Code: 7, Kind: "drift_detected", Retryable: false,
			Description: "drift check found differences between two snapshots"},
	}
}

// walkCommandTree recursively visits every command in the tree and returns a
// flat slice of CommandDoc values in depth-first order. The root command itself
// is not included; only its descendants are.
//
// parentPath is the accumulated space-joined path of command words above this
// level (not including the root program name "zscalerctl"). It is "" for
// direct children of root.
func walkCommandTree(cmd *cobra.Command, parentPath string) []CommandDoc {
	var docs []CommandDoc
	for _, sub := range cmd.Commands() {
		docs = append(docs, buildCommandDocs(sub, parentPath)...)
	}
	return docs
}

// buildCommandDocs produces a CommandDoc for cmd and recurses into its children.
// For product commands (zia, zpa, ztw, zcc, zidentity), instead of emitting a
// single "zia" entry, it synthesizes virtual CommandDoc entries per
// {product} {resource} {op} triple from the catalog. This matches the spec's
// intended shape: "zia locations list", "zia locations get", etc., so agents
// can map each invocation form to its output_fields and args.
func buildCommandDocs(cmd *cobra.Command, parentPath string) []CommandDoc {
	// Compute this command's path (space-joined, no root program name).
	var fullPath string
	if parentPath == "" {
		fullPath = cmd.Name()
	} else {
		fullPath = parentPath + " " + cmd.Name()
	}

	// Product commands are identified by being a direct child of the root
	// (parentPath == "") whose name matches a known product. For these, we
	// synthesize virtual entries from the catalog instead of treating "zia"
	// as a leaf command, because the spec's target shape is "zia locations list".
	if parentPath == "" {
		if isKnownProductName(cmd.Name()) {
			var docs []CommandDoc
			// First, include any real Cobra subcommands of the product (e.g. url-lookup).
			for _, sub := range cmd.Commands() {
				docs = append(docs, buildCommandDocs(sub, fullPath)...)
			}
			// Then synthesize virtual entries for each catalog resource × read-op.
			docs = append(docs, buildProductResourceDocs(cmd, fullPath)...)
			return docs
		}
	}

	doc := CommandDoc{
		Path:           fullPath,
		Short:          cmd.Short,
		Long:           strings.TrimSpace(cmd.Long),
		Aliases:        cmd.Aliases,
		Hidden:         cmd.Hidden,
		Deprecated:     cmd.Deprecated,
		Mutating:       cmd.Annotations["introspect/mutating"] == "true",
		Args:           buildArgsDoc(cmd),
		Flags:          buildLocalFlagDocs(cmd),
		InheritedFlags: buildInheritedFlagNames(cmd),
		OutputFields:   nil, // non-product commands have no catalog output fields
	}
	if doc.Aliases == nil {
		doc.Aliases = []string{}
	}

	var docs []CommandDoc
	docs = append(docs, doc)
	// Recurse into children.
	for _, sub := range cmd.Commands() {
		docs = append(docs, buildCommandDocs(sub, fullPath)...)
	}
	return docs
}

// isKnownProductName reports whether name is a known product in the catalog.
// This avoids importing knownProducts() directly (it's an internal helper).
func isKnownProductName(name string) bool {
	for _, spec := range resources.Catalog() {
		if string(spec.Product) == name {
			return true
		}
	}
	return false
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
				Flags:          nil,
				InheritedFlags: inheritedNames,
				OutputFields:   fields,
			}
			docs = append(docs, doc)
		}
	}
	return docs
}

// buildArgsDoc derives an ArgsDoc from the command's ValidArgs and Use string.
// The policy is best-effort: "none", "exact", "range", or "arbitrary".
func buildArgsDoc(cmd *cobra.Command) ArgsDoc {
	if len(cmd.ValidArgs) > 0 {
		return ArgsDoc{
			Policy:      "exact",
			N:           1,
			ValidValues: cmd.ValidArgs,
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
	var docs []FlagDoc
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
	var names []string
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		names = append(names, f.Name)
	})
	return names
}

// WalkCobraTree visits each non-root command in the Cobra tree rooted at root,
// calling fn(cmd, path) for each one in depth-first, alphabetically-sorted
// order (matching gen-cli-docs sort order). path is the space-joined command
// path without the root program name (e.g. "config", "config init").
//
// This is the shared enumeration used by both the markdown docs generator
// (scripts/gen-cli-docs.go) and IntrospectTree. Walking from the same function
// ensures docs and the agent JSON map always enumerate the identical command set.
func WalkCobraTree(root *cobra.Command, fn func(cmd *cobra.Command, path string)) {
	walkCobraSubtree(root, "", fn)
}

func walkCobraSubtree(cmd *cobra.Command, parentPath string, fn func(*cobra.Command, string)) {
	subs := cmd.Commands()
	// Sort alphabetically — matches gen-cli-docs sort order (deterministic).
	sortedSubs := make([]*cobra.Command, len(subs))
	copy(sortedSubs, subs)
	for i := 1; i < len(sortedSubs); i++ {
		for j := i; j > 0 && sortedSubs[j].Name() < sortedSubs[j-1].Name(); j-- {
			sortedSubs[j], sortedSubs[j-1] = sortedSubs[j-1], sortedSubs[j]
		}
	}
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
