package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/redact"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, os.Environ()))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, env []string) int {
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
