//go:build ignore

// Command live-smoke runs a read-only live smoke against the configured
// zscalerctl credentials. It is the Go port of scripts/live-smoke.sh; the
// validation logic lives in internal/livesmoke and is covered by go test.
//
// Usage:
//
//	go run ./scripts/live-smoke.go [--out DIR] [--bin PATH] [--resources LIST]
//	    [--manifest FILE] [--no-manifest] [--require-credentials]
//	    [--require-nonempty] [--strict-counts] [--skip-credential-check]
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/livesmoke"
)

const usage = `usage: go run ./scripts/live-smoke.go [--out DIR] [--bin PATH] [--resources LIST] [--manifest FILE] [--no-manifest] [--require-credentials] [--require-nonempty] [--strict-counts]

Runs a read-only live smoke against the currently configured zscalerctl
credentials and prints PASS/FAIL markers for pre-PR validation. By default all
current ZIA read resources are validated; a manifest or --resources value may
select qualified non-ZIA resources such as ztw/workload-groups.

This command does not print credential values or live resource payloads. It
recognizes explicit zscalerctl OneAPI or ZIA legacy credentials. Non-ZIA
resources require OneAPI credentials; selected ZPA resources also require
ZSCALERCTL_ZPA_CUSTOMER_ID. Raw SDK env vars are intentionally ignored.
`

func main() {
	args := os.Args[1:]
	opts := livesmoke.Options{Bin: os.Getenv("ZSCALERCTL_BIN")}

	need := func(i int, flag string) string {
		if i+1 >= len(args) {
			fmt.Fprintf(os.Stderr, "%s requires a value\n", flag)
			os.Exit(2)
		}
		return args[i+1]
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			opts.OutDir = need(i, "--out")
			i++
		case "--bin":
			opts.Bin = need(i, "--bin")
			i++
		case "--resources":
			opts.Resources = append(opts.Resources, strings.Split(need(i, "--resources"), ",")...)
			i++
		case "--manifest":
			opts.ManifestPath = need(i, "--manifest")
			i++
		case "--no-manifest":
			opts.NoManifest = true
		case "--require-credentials":
			opts.RequireCredentials = true
		case "--require-nonempty":
			opts.RequireNonempty = true
		case "--strict-counts":
			opts.StrictCounts = true
		case "--skip-credential-check":
			opts.SkipCredentialCheck = true
		case "-h", "--help":
			fmt.Print(usage)
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "unknown argument: %s\n%s", args[i], usage)
			os.Exit(2)
		}
	}

	runner := livesmoke.NewExecRunner(opts.Bin)
	os.Exit(livesmoke.Run(opts, os.Getenv, runner, os.Stdout, os.Stderr))
}
