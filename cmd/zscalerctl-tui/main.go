// Command zscalerctl-tui is an experimental standalone TUI browser binary.
//
// It is intentionally separate from the normal zscalerctl binary so that
// Bubble Tea (which runs terminal probing at package init) is never linked into
// the main CLI. This binary may import internal/tui/tea and Bubble Tea freely.
//
// Current modes are fixture-only: no config, credentials, readers, or network are
// used. Use --collector-fixture to exercise the collector path, or --fixture to
// use the hard-coded fake fixture.
//
// Usage:
//
//	go run ./cmd/zscalerctl-tui [--collector-fixture] [--fixture] [--color auto|always|never] [--format auto|table|pretty|json|ndjson]
//
// The default mode is --collector-fixture.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/tui"
	"github.com/dvmrry/zscalerctl/internal/tui/browserdata"
	"github.com/dvmrry/zscalerctl/internal/tui/data"
	tui_tea "github.com/dvmrry/zscalerctl/internal/tui/tea"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "zscalerctl-tui: %v\n", err)
		os.Exit(2)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("zscalerctl-tui", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	fixtureFlag := flags.Bool("fixture", false, "use the hard-coded fake fixture")
	collectorFixtureFlag := flags.Bool("collector-fixture", false, "use the fake-reader-backed collector fixture")
	colorFlag := flags.String("color", string(output.ColorAuto), "color mode: auto, always, never")
	formatFlag := flags.String("format", string(output.FormatAuto), "output format gate: auto, table, pretty, json, ndjson")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	if *fixtureFlag && *collectorFixtureFlag {
		return fmt.Errorf("--fixture and --collector-fixture are mutually exclusive")
	}

	colorMode, err := output.ParseColorMode(*colorFlag)
	if err != nil {
		return err
	}
	format, err := output.ParseFormat(*formatFlag)
	if err != nil {
		return err
	}

	env := os.Environ()
	stdinTTY := output.IsTerminal(os.Stdin)
	stdoutTTY := output.IsTerminal(os.Stdout)
	stderrTTY := output.IsTerminal(os.Stderr)

	eligibility := tui.Evaluate(tui.Options{
		Requested:  true,
		StdinTTY:   stdinTTY,
		StdoutTTY:  stdoutTTY,
		StderrTTY:  stderrTTY,
		Format:     format,
		ColorMode:  colorMode,
		OutputPath: "",
		Env:        env,
	})
	if !eligibility.Enabled {
		return fmt.Errorf("disabled: %s", eligibility.Reason)
	}

	var browserData data.BrowserData
	if *fixtureFlag {
		browserData = data.NewFakeBrowserData()
	} else {
		// Default mode: collector fixture.
		collector := browserdata.NewCollectorFixture()
		browserData, err = collector.Collect(ctx, browserdata.CollectOptions{ContinueOnError: true})
		if err != nil {
			return err
		}
	}

	style := output.NewStyle(
		output.ShouldColor(colorMode, env, stdoutTTY),
		output.Supports256Color(env),
	)
	style.Width = output.TerminalWidth(os.Stdout)

	program := tea.NewProgram(
		tui_tea.NewBrowserModel(style, browserData),
		tea.WithContext(ctx),
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	)
	_, err = program.Run()
	return err
}
