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
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

var processOutputMu sync.Mutex

const (
	exitSuccess           = 0
	exitInternalError     = 1
	exitUsageError        = 2
	exitCredentialError   = 3
	exitNotFound          = 4
	exitLiveAccessFailure = 5
	exitPartialDump       = 6
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, os.Environ()))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, env []string) (exitCode int) {
	processOutputMu.Lock()
	defer processOutputMu.Unlock()

	restoreProcessOutput, err := muteProcessOutput()
	if err != nil {
		writeError(stderr, output.FormatTable, fmt.Errorf("internal error: %w", err))
		return exitInternalError
	}
	defer restoreProcessOutput()
	defer func() {
		if recovered := recover(); recovered != nil {
			writeError(stderr, errorFormat(args, stdout), fmt.Errorf("internal error: %v", recovered))
			exitCode = exitInternalError
		}
	}()

	app := cli.New(stdout, stderr, env)
	if err := app.Run(ctx, args); err != nil {
		code := exitCodeForError(err)
		format := errorFormat(args, stdout)
		if errors.Is(err, cli.ErrPartialDump) && format != output.FormatJSON {
			return code
		}
		writeError(stderr, format, err)
		return code
	}
	return exitSuccess
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

func (errorEnvelope) OutputSafe() {}

type errorBody struct {
	Kind      string `json:"kind"`
	Message   string `json:"message"`
	Product   string `json:"product,omitempty"`
	Resource  string `json:"resource,omitempty"`
	Operation string `json:"operation,omitempty"`
}

// errorFormat decides how a command-boundary error is rendered, keeping it
// consistent with the data path: an explicit --format json (or the default
// auto resolving to JSON because stdout is not a terminal) gets the JSON
// envelope; explicit table/pretty, or auto on an interactive terminal, gets a
// plain-text line. Without this, `--format auto` (the default) emitted JSON
// data but text errors on the same pipe.
func errorFormat(args []string, stdout io.Writer) output.Format {
	switch cli.RequestedFormatRaw(args) {
	case output.FormatJSON:
		return output.FormatJSON
	case output.FormatAuto:
		if !output.IsTerminal(stdout) {
			return output.FormatJSON
		}
		return output.FormatTable
	default:
		return output.FormatTable
	}
}

func writeError(w io.Writer, format output.Format, err error) {
	if format == output.FormatJSON {
		envelope := errorEnvelope{Error: errorDetails(err)}
		_ = output.NewRenderer(redact.New(redact.ModeStandard)).WriteJSON(w, envelope)
		return
	}
	message := redact.New(redact.ModeStandard).String(err.Error())
	fmt.Fprintf(w, "zscalerctl: %s\n", message)
}

func errorDetails(err error) errorBody {
	body := errorBody{
		Kind:    errorKind(err),
		Message: err.Error(),
	}
	var notFound cli.ResourceNotFoundError
	if errors.As(err, &notFound) {
		body.Product = string(notFound.Product)
		body.Resource = notFound.Resource
	}
	return body
}

func errorKind(err error) string {
	switch {
	case errors.Is(err, cli.ErrUsage):
		return "usage"
	case errors.Is(err, cli.ErrPartialDump):
		return "partial_dump"
	case errors.Is(err, cli.ErrNotFound):
		return "not_found"
	case errors.Is(err, zscaler.ErrMissingCredentials):
		return "missing_credentials"
	case errors.Is(err, zscaler.ErrInvalidResourceID):
		return "invalid_resource_id"
	case errors.Is(err, zscaler.ErrUnsupportedResource):
		return "unsupported_resource"
	case errors.Is(err, zscaler.ErrLiveAccessFailed):
		return "live_access_failed"
	case errors.Is(err, zscaler.ErrInvalidProxyConfig):
		return "invalid_proxy_config"
	case errors.Is(err, config.ErrInvalidConfig):
		return "invalid_config"
	default:
		return "internal"
	}
}

func exitCodeForError(err error) int {
	switch {
	case errors.Is(err, cli.ErrUsage):
		return exitUsageError
	case errors.Is(err, cli.ErrPartialDump):
		return exitPartialDump
	case errors.Is(err, cli.ErrNotFound):
		return exitNotFound
	case errors.Is(err, zscaler.ErrMissingCredentials):
		return exitCredentialError
	case errors.Is(err, zscaler.ErrInvalidResourceID):
		return exitUsageError
	case errors.Is(err, zscaler.ErrUnsupportedResource):
		return exitNotFound
	case errors.Is(err, zscaler.ErrLiveAccessFailed):
		return exitLiveAccessFailure
	case errors.Is(err, zscaler.ErrInvalidProxyConfig):
		return exitUsageError
	case errors.Is(err, config.ErrInvalidConfig):
		return exitUsageError
	default:
		return exitInternalError
	}
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
