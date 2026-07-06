package cli

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/spf13/cobra"
)

type helpRenderer struct {
	style output.Style

	heading lipgloss.Style
	usage   lipgloss.Style
	command lipgloss.Style
	flag    lipgloss.Style
}

func newHelpRenderer(style output.Style) helpRenderer {
	r := helpRenderer{style: style}
	if !style.Color {
		return r
	}
	accent := lipgloss.Color("6")
	muted := lipgloss.Color("8")
	if style.Color256 {
		accent = lipgloss.Color("45")
		muted = lipgloss.Color("245")
	}
	r.heading = lipgloss.NewStyle().Bold(true).Foreground(accent)
	r.usage = lipgloss.NewStyle().Foreground(accent)
	r.command = lipgloss.NewStyle().Bold(true).Foreground(accent)
	r.flag = lipgloss.NewStyle().Foreground(muted)
	return r
}

func configureHelp(root *cobra.Command, style output.Style) {
	renderer := newHelpRenderer(style)
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if err := renderer.writeHelp(cmd.OutOrStdout(), cmd); err != nil {
			cmd.PrintErrln(err)
		}
	})
	root.SetUsageFunc(func(cmd *cobra.Command) error {
		return renderer.writeUsage(cmd.OutOrStdout(), cmd)
	})
}

func (r helpRenderer) writeHelp(w io.Writer, cmd *cobra.Command) error {
	var b strings.Builder
	usage := strings.TrimRightFunc(firstNonEmpty(cmd.Long, cmd.Short), unicode.IsSpace)
	if usage != "" {
		b.WriteString(usage)
		b.WriteString("\n\n")
	}
	if cmd.Runnable() || cmd.HasSubCommands() {
		r.writeUsageBlock(&b, cmd)
	}
	_, err := io.WriteString(w, r.filterANSI(b.String()))
	return err
}

func (r helpRenderer) writeUsage(w io.Writer, cmd *cobra.Command) error {
	var b strings.Builder
	r.writeUsageBlock(&b, cmd)
	_, err := io.WriteString(w, r.filterANSI(b.String()))
	return err
}

func (r helpRenderer) writeUsageBlock(b *strings.Builder, cmd *cobra.Command) {
	fmt.Fprintf(b, "%s", r.headingText("Usage:"))
	if cmd.Runnable() {
		fmt.Fprintf(b, "\n  %s", r.usageText(cmd.UseLine()))
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(b, "\n  %s", r.usageText(cmd.CommandPath()+" [command]"))
	}
	if len(cmd.Aliases) > 0 {
		fmt.Fprintf(b, "\n\n%s\n", r.headingText("Aliases:"))
		fmt.Fprintf(b, "  %s", cmd.NameAndAliases())
	}
	if cmd.HasExample() {
		fmt.Fprintf(b, "\n\n%s\n", r.headingText("Examples:"))
		fmt.Fprintf(b, "%s", cmd.Example)
	}
	if cmd.HasAvailableSubCommands() {
		r.writeCommands(b, cmd)
	}
	if cmd.HasAvailableLocalFlags() {
		fmt.Fprintf(b, "\n\n%s\n", r.headingText("Flags:"))
		b.WriteString(r.flagUsages(cmd.LocalFlags().FlagUsages()))
	}
	if cmd.HasAvailableInheritedFlags() {
		fmt.Fprintf(b, "\n\n%s\n", r.headingText("Global Flags:"))
		b.WriteString(r.flagUsages(cmd.InheritedFlags().FlagUsages()))
	}
	if cmd.HasHelpSubCommands() {
		fmt.Fprintf(b, "\n\n%s", r.headingText("Additional help topics:"))
		for _, subcmd := range cmd.Commands() {
			if subcmd.IsAdditionalHelpTopicCommand() {
				fmt.Fprintf(
					b,
					"\n  %s %s",
					r.paddedCommand(subcmd.CommandPath(), subcmd.CommandPathPadding()),
					subcmd.Short,
				)
			}
		}
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(
			b,
			"\n\nUse \"%s\" for more information about a command.",
			r.usageText(cmd.CommandPath()+" [command] --help"),
		)
	}
	b.WriteByte('\n')
}

func (r helpRenderer) writeCommands(b *strings.Builder, cmd *cobra.Command) {
	cmds := cmd.Commands()
	if len(cmd.Groups()) == 0 {
		fmt.Fprintf(b, "\n\n%s", r.headingText("Available Commands:"))
		for _, subcmd := range cmds {
			if subcmd.IsAvailableCommand() || subcmd.Name() == "help" {
				fmt.Fprintf(b, "\n  %s %s", r.paddedCommand(subcmd.Name(), subcmd.NamePadding()), subcmd.Short)
			}
		}
		return
	}

	for _, group := range cmd.Groups() {
		fmt.Fprintf(b, "\n\n%s", r.headingText(group.Title))
		for _, subcmd := range cmds {
			if subcmd.GroupID == group.ID && (subcmd.IsAvailableCommand() || subcmd.Name() == "help") {
				fmt.Fprintf(b, "\n  %s %s", r.paddedCommand(subcmd.Name(), subcmd.NamePadding()), subcmd.Short)
			}
		}
	}
	if cmd.AllChildCommandsHaveGroup() {
		return
	}
	fmt.Fprintf(b, "\n\n%s", r.headingText("Additional Commands:"))
	for _, subcmd := range cmds {
		if subcmd.GroupID == "" && (subcmd.IsAvailableCommand() || subcmd.Name() == "help") {
			fmt.Fprintf(b, "\n  %s %s", r.paddedCommand(subcmd.Name(), subcmd.NamePadding()), subcmd.Short)
		}
	}
}

func (r helpRenderer) renderResourceUsage(product resources.Product, spec resources.ResourceSpec, width int) string {
	if !r.style.Color {
		return resourceUsage(product, spec, width)
	}
	var b strings.Builder
	fmt.Fprintf(
		&b,
		"%s %s",
		r.headingText("usage:"),
		r.usageText(fmt.Sprintf("zscalerctl %s %s %s", product, spec.Name, strings.Join(readOperationNames(spec), "|"))),
	)
	if fields := spec.FieldOrder(redact.ModeStandard); len(fields) > 0 {
		b.WriteString("\n")
		b.WriteString(r.headingText("fields:"))
		b.WriteByte('\n')
		b.WriteString(columnize(fields, width))
	}
	return r.filterANSI(b.String())
}

func (r helpRenderer) flagUsages(raw string) string {
	raw = strings.TrimRightFunc(raw, unicode.IsSpace)
	if !r.style.Color {
		return raw
	}
	return helpFlagPattern.ReplaceAllStringFunc(raw, func(match string) string {
		if strings.HasPrefix(match, "-") {
			return r.flag.Render(match)
		}
		return match[:1] + r.flag.Render(match[1:])
	})
}

func (r helpRenderer) paddedCommand(name string, padding int) string {
	spaces := padding - len(name)
	if spaces < 0 {
		spaces = 0
	}
	if !r.style.Color {
		return name + strings.Repeat(" ", spaces)
	}
	return r.command.Render(name) + strings.Repeat(" ", spaces)
}

func (r helpRenderer) headingText(value string) string {
	if !r.style.Color {
		return value
	}
	return r.heading.Render(value)
}

func (r helpRenderer) usageText(value string) string {
	if !r.style.Color {
		return value
	}
	return r.usage.Render(value)
}

func (r helpRenderer) filterANSI(rendered string) string {
	if !r.style.Color {
		return rendered
	}
	var buf bytes.Buffer
	w := colorprofile.Writer{
		Forward: &buf,
		Profile: helpColorProfile(r.style),
	}
	_, _ = w.WriteString(rendered)
	return buf.String()
}

func helpColorProfile(style output.Style) colorprofile.Profile {
	if !style.Color {
		return colorprofile.NoTTY
	}
	if style.Color256 {
		return colorprofile.ANSI256
	}
	return colorprofile.ANSI
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

var helpFlagPattern = regexp.MustCompile(`(^|[\s,])(-{1,2}[A-Za-z0-9][A-Za-z0-9-]*)`)
