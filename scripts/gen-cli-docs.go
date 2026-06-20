//go:build ignore

// gen-cli-docs generates a Markdown CLI reference from the live Cobra command
// tree and writes it to docs/cli/zscalerctl.md.
//
// Usage:
//
//	go run ./scripts/gen-cli-docs.go [--out docs/cli/zscalerctl.md]
//
// The output is deterministic (sorted subcommands, no timestamps) so it can be
// committed and drift-gated: scripts/verify-cli-docs.sh regenerates to a temp
// location and git-diffs the result against the committed file.
//
// Design constraints:
//   - Zero additional dependencies: walks the tree with Cobra's introspection
//     API only (cmd.Commands(), cmd.Use, cmd.Short, cmd.Long, cmd.Example,
//     cmd.Flags(), cmd.LocalFlags(), cmd.InheritedFlags(), cmd.Aliases).
//   - No timestamps or other non-deterministic output.
//   - Config-free: BuildCommandTree uses zero-value globalOptions so no
//     credentials or config file are needed.

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func main() {
	outFlag := flag.String("out", filepath.Join("docs", "cli", "zscalerctl.md"), "output file path")
	flag.Parse()

	app := cli.New(io.Discard, io.Discard, nil)
	root := cli.BuildCommandTree(app)

	var sb strings.Builder
	writeDoc(&sb, root)

	outPath := *outFlag
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gen-cli-docs: mkdir %s: %v\n", filepath.Dir(outPath), err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, []byte(sb.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "gen-cli-docs: write %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "gen-cli-docs: wrote %s\n", outPath)
}

// writeDoc emits the full CLI reference into w, starting with a global preamble
// then one section per command in depth-first, alphabetically-sorted order.
func writeDoc(w io.StringWriter, root *cobra.Command) {
	writeLines(w,
		"# zscalerctl CLI Reference",
		"",
		"This reference is generated from the live Cobra command tree.",
		"Do not edit by hand — run `go run ./scripts/gen-cli-docs.go` to regenerate.",
		"",
		"## Global Flags",
		"",
		"These flags are accepted by every command:",
		"",
	)
	writeFlagTable(w, root.PersistentFlags())
	writeLines(w, "")

	writeLines(w, "## Commands", "")
	// depth=3 → "###" for top-level subcommands; path="" means no prefix yet.
	writeCommandSections(w, root, "", 3)
}

// writeCommandSections writes a section for each non-hidden subcommand of cmd,
// recursing depth-first.
//
// path is the space-separated command words accumulated so far (not including
// "zscalerctl"); e.g. "" at the top level, "config" for config's children.
// depth is the Markdown heading level (### = 3) for this tier.
func writeCommandSections(w io.StringWriter, cmd *cobra.Command, path string, depth int) {
	subs := cmd.Commands()
	// Deterministic order: sort by command name.
	sort.Slice(subs, func(i, j int) bool {
		return subs[i].Name() < subs[j].Name()
	})
	for _, sub := range subs {
		if sub.Hidden {
			continue
		}
		writeCommandSection(w, sub, path, depth)
	}
}

// writeCommandSection emits one Markdown section for cmd, then recurses into
// its non-hidden subcommands.
//
// path is the accumulated parent path (space-separated command words, not
// including "zscalerctl"); e.g. "" for top-level, "config" for config's children.
// depth is the Markdown heading level for this tier (3 = ###).
func writeCommandSection(w io.StringWriter, cmd *cobra.Command, path string, depth int) {
	// Full path: e.g. "config" at top level, "config init" one level down.
	var fullPath string
	if path == "" {
		fullPath = cmd.Name()
	} else {
		fullPath = path + " " + cmd.Name()
	}

	heading := strings.Repeat("#", depth) + " " + fullPath

	writeLines(w, heading, "")

	if cmd.Short != "" {
		writeLines(w, cmd.Short, "")
	}

	// Usage line: "zscalerctl <fullPath> [args...]"
	suffix := usageSuffix(cmd)
	usageLine := "zscalerctl " + fullPath
	if suffix != "" {
		usageLine += " " + suffix
	}
	writeLines(w, "```", usageLine, "```", "")

	if cmd.Long != "" {
		writeLines(w, cmd.Long, "")
	}

	if len(cmd.Aliases) > 0 {
		writeLines(w, "**Aliases:** "+strings.Join(cmd.Aliases, ", "), "")
	}

	if cmd.Example != "" {
		writeLines(w, "**Examples:**", "", "```sh", strings.TrimRight(cmd.Example, "\n"), "```", "")
	}

	// Local flags (not inherited, not persistent from parent).
	local := cmd.LocalFlags()
	if hasFlags(local) {
		writeLines(w, "**Flags:**", "")
		writeFlagTable(w, local)
		writeLines(w, "")
	}

	// Recurse into subcommands at the next heading level.
	writeCommandSections(w, cmd, fullPath, depth+1)
}

// usageSuffix extracts the argument/operand portion of cmd.Use (everything
// after the first word, which is the command name).
func usageSuffix(cmd *cobra.Command) string {
	use := strings.TrimSpace(cmd.Use)
	idx := strings.Index(use, " ")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(use[idx:])
}

// hasFlags reports whether fs has any flags at all.
func hasFlags(fs *pflag.FlagSet) bool {
	has := false
	fs.VisitAll(func(_ *pflag.Flag) { has = true })
	return has
}

// writeFlagTable emits a Markdown table of flag name, type, default, and usage
// for every flag in fs, sorted by flag name.
func writeFlagTable(w io.StringWriter, fs *pflag.FlagSet) {
	type row struct {
		name       string
		kind       string
		defaultVal string
		usage      string
	}
	var rows []row
	fs.VisitAll(func(f *pflag.Flag) {
		def := f.DefValue
		if f.Value.Type() == "stringArray" && def == "[]" {
			def = ""
		}
		rows = append(rows, row{
			name:       "--" + f.Name,
			kind:       f.Value.Type(),
			defaultVal: def,
			usage:      f.Usage,
		})
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	writeLines(w, "| Flag | Type | Default | Description |", "| --- | --- | --- | --- |")
	for _, r := range rows {
		def := r.defaultVal
		if def == "" {
			def = "—"
		}
		writeLines(w, fmt.Sprintf("| `%s` | `%s` | `%s` | %s |",
			r.name, r.kind, def, escapeMarkdown(r.usage)))
	}
}

// escapeMarkdown escapes pipe characters so they do not break Markdown tables.
func escapeMarkdown(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

// writeLines writes each string followed by a newline.
func writeLines(w io.StringWriter, lines ...string) {
	for _, line := range lines {
		_, _ = w.WriteString(line + "\n")
	}
}
