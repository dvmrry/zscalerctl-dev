package main

import (
	"context"
	"flag"
	"io"
	"os"
	"time"

	"github.com/dvmrry/zscalerctl/internal/enginehost"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	machineruntime "github.com/dvmrry/zscalerctl/internal/runtime"
	"github.com/dvmrry/zscalerctl/internal/version"
)

const (
	exitSuccess  = 0
	exitInternal = 1
	exitProtocol = 2
)

func main() {
	configureBrokenPipe()
	stdin, stdout, err := processStreams(os.Stdin, os.Stdout)
	if err != nil {
		os.Exit(exitInternal)
	}
	controller := newSignalController(context.Background(), os.Exit)
	code := run(
		controller.Context(), os.Args[1:],
		stdin, stdout, os.Environ(),
	)
	controller.Stop()
	if signalCode := controller.ExitCode(); signalCode != 0 {
		code = signalCode
	}
	os.Exit(code)
}

type interruptibleInput struct {
	*os.File
}

func (input interruptibleInput) Close() error {
	_ = input.SetReadDeadline(time.Now())
	return input.File.Close()
}

type interruptibleOutput struct {
	*os.File
}

func (output interruptibleOutput) Close() error {
	_ = output.SetWriteDeadline(time.Now())
	return output.File.Close()
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, env []string) int {
	settings, ok := parseSettings(args)
	if !ok {
		return exitProtocol
	}
	opts := machineruntime.OptionsFromExecutionSettings(settings)
	opts.Env = append([]string(nil), env...)
	engine, err := machineruntime.NewEngine(opts)
	if err != nil {
		return exitInternal
	}
	host, err := enginehost.New(engine, version.Current().Version)
	if err != nil {
		return exitInternal
	}
	err = host.Serve(ctx, enginehost.Streams{Input: stdin, Output: stdout})
	return enginehost.ExitCode(err)
}

func parseSettings(args []string) (machine.ExecutionSettings, bool) {
	flags := flag.NewFlagSet("zscalerctl-engine", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	profile := flags.String("profile", "", "profile name")
	configPath := flags.String("config", "", "config file path")
	timeout := flags.Duration("timeout", 30*time.Second, "request timeout")
	redactionValue := flags.String("redaction", "", "redaction mode")
	noCache := flags.Bool("no-cache", false, "bypass API cache")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *timeout <= 0 {
		return machine.ExecutionSettings{}, false
	}
	settings := machine.ExecutionSettings{
		Profile: *profile, ConfigPath: *configPath, Timeout: *timeout, NoCache: *noCache,
	}
	if *redactionValue != "" {
		mode, err := redact.ParseMode(*redactionValue)
		if err != nil {
			return machine.ExecutionSettings{}, false
		}
		settings.Redaction = mode
		settings.RedactionSet = true
	}
	return settings, true
}
