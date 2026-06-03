package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/redact"
)

var processOutputMu sync.Mutex

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, os.Environ()))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, env []string) (exitCode int) {
	processOutputMu.Lock()
	defer processOutputMu.Unlock()

	restoreProcessOutput, err := muteProcessOutput()
	if err != nil {
		message := redact.New(redact.ModeStandard).String(err.Error())
		fmt.Fprintf(stderr, "zscalerctl: internal error: %s\n", message)
		return 1
	}
	defer restoreProcessOutput()
	defer func() {
		if recovered := recover(); recovered != nil {
			message := redact.New(redact.ModeStandard).String(fmt.Sprint(recovered))
			fmt.Fprintf(stderr, "zscalerctl: internal error: %s\n", message)
			exitCode = 1
		}
	}()

	app := cli.New(stdout, stderr, env)
	if err := app.Run(ctx, args); err != nil {
		message := redact.New(redact.ModeStandard).String(err.Error())
		fmt.Fprintf(stderr, "zscalerctl: %s\n", message)
		if errors.Is(err, cli.ErrUsage) {
			return 2
		}
		return 1
	}
	return 0
}

func muteProcessOutput() (func(), error) {
	previousLogWriter := log.Writer()
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open null output sink: %w", err)
	}
	previousStdout := os.Stdout
	log.SetOutput(io.Discard)
	os.Stdout = devNull

	return func() {
		os.Stdout = previousStdout
		log.SetOutput(previousLogWriter)
		_ = devNull.Close()
	}, nil
}
