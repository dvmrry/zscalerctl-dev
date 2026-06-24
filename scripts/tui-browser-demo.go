//go:build ignore

// Command tui-browser-demo runs a development-only static TUI browser demo.
//
// Usage:
//
//	go run ./scripts/tui-browser-demo.go [--color auto|always|never] [--format auto|table|pretty|json|ndjson]
//
// This harness intentionally does not load zscalerctl config, resolve
// credentials, contact Zscaler, run subprocesses, write files, or add CLI
// command surface. It uses only hard-coded fake data to prove the product
// browser shape on the feature/tui integration branch.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/tui"
	tui_tea "github.com/dvmrry/zscalerctl/internal/tui/tea"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "tui browser demo: %v\n", err)
		os.Exit(2)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("tui-browser-demo", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	colorFlag := flags.String("color", string(output.ColorAuto), "color mode: auto, always, never")
	formatFlag := flags.String("format", string(output.FormatAuto), "output format gate: auto, table, pretty, json, ndjson")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
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
		Requested: true,
		StdinTTY:  stdinTTY,
		StdoutTTY: stdoutTTY,
		StderrTTY: stderrTTY,
		Format:    format,
		ColorMode: colorMode,
		Env:       env,
	})
	if !eligibility.Enabled {
		return fmt.Errorf("disabled: %s", eligibility.Reason)
	}

	style := output.NewStyle(
		output.ShouldColor(colorMode, env, stdoutTTY),
		output.Supports256Color(env),
	)
	style.Width = output.TerminalWidth(os.Stdout)

	program := tea.NewProgram(
		tui_tea.NewBrowserModel(style),
		tea.WithContext(ctx),
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	)
	_, err = program.Run()
	return err
}
