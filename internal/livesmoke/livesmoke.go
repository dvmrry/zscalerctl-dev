package livesmoke

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Run executes a live smoke and returns a process exit code: 0 success or
// credential-skip, 1 a validation/credential failure, 2 a usage/setup error.
// out receives PASS/INFO/SKIP markers and the result table; errw receives FAIL
// markers and the failure summary. runner executes the CLI; env resolves
// credentials.
func Run(opts Options, env Env, runner Runner, out, errw io.Writer) int {
	if env == nil {
		env = osEnv
	}
	s := &smoke{opts: opts, env: env, runner: runner, rep: &reporter{out: out, errw: errw}, listCounts: map[string]int{}}
	return s.run()
}

type smoke struct {
	opts       Options
	env        Env
	runner     Runner
	rep        *reporter
	specs      []ResourceSpec
	requested  []string
	resources  []string
	outDir     string
	stderrs    []namedStderr
	listCounts map[string]int
}

func (s *smoke) usage(format string, a ...any) int {
	fmt.Fprintf(s.rep.errw, format+"\n", a...)
	return 2
}

func (s *smoke) run() int {
	// 1. Resolve the requested resource filter (explicit --resources, an explicit
	// manifest, or default-manifest auto-discovery on a feature branch).
	if len(s.opts.Resources) > 0 {
		for _, entry := range s.opts.Resources {
			norm, err := normalizeRequestedResource(entry)
			if err != nil {
				return s.usage("%v", err)
			}
			s.requested = append(s.requested, norm)
		}
	}
	switch {
	case s.opts.ManifestPath != "":
		entries, err := s.loadManifest(s.opts.ManifestPath)
		if err != nil {
			return s.usage("%v", err)
		}
		s.requested = append(s.requested, entries...)
		s.rep.info("using live smoke manifest: %s", s.opts.ManifestPath)
	case s.shouldUseDefaultManifest():
		entries, err := s.loadManifest(defaultManifest)
		if err != nil {
			return s.usage("%v", err)
		}
		s.requested = append(s.requested, entries...)
		s.rep.info("using live smoke manifest: %s", defaultManifest)
	}

	// 2. Credential preflight.
	if !s.opts.SkipCredentialCheck {
		if family := credentialFamily(s.env); family != "" {
			s.rep.pass("live credential preflight found %s credentials", family)
		} else {
			const msg = "no supported live credentials configured; set explicit zscalerctl OneAPI or ZIA legacy env vars"
			if s.opts.RequireCredentials {
				s.rep.fail(msg)
				return 1
			}
			s.rep.skip(msg)
			return 0
		}
	}

	// 3. Verify the binary exists (exec runner only) and set up the output dir.
	if s.opts.Bin != "" {
		// #nosec G204 -- operator-supplied --bin path for this dev tool.
		if _, err := exec.LookPath(s.opts.Bin); err != nil {
			return s.usage("zscalerctl binary not found or not executable: %s", s.opts.Bin)
		}
	}
	if code := s.setupOutDir(); code != 0 {
		return code
	}
	s.rep.info("artifacts: %s", s.outDir)

	// 4. Load the catalog and select resources.
	if !s.loadResources() {
		return s.finishFailure()
	}

	// 5. Credential scope guards for the selected products.
	if !s.opts.SkipCredentialCheck {
		if s.selectedHasNonZIA() {
			if s.env("ZSCALERCTL_AUTH_MODE") == "zia-legacy" ||
				!isSet(s.env, "ZSCALERCTL_CLIENT_ID") ||
				(!isSet(s.env, "ZSCALERCTL_CLIENT_SECRET") && !isSet(s.env, "ZSCALERCTL_CLIENT_SECRET_FILE")) ||
				!isSet(s.env, "ZSCALERCTL_VANITY_DOMAIN") {
				s.rep.fail("selected non-ZIA resources require OneAPI credentials")
				return s.finishFailure()
			}
		}
		if s.selectedHasProduct("zpa") && !isSet(s.env, "ZSCALERCTL_ZPA_CUSTOMER_ID") {
			s.rep.fail("selected ZPA resources require ZSCALERCTL_ZPA_CUSTOMER_ID")
			return s.finishFailure()
		}
	}

	// 6. Per-resource list/show validation.
	for _, qualified := range s.resources {
		s.validateResource(qualified)
	}

	// 7. Dump and dump validation.
	s.validateDump()

	// 8. Result table and exit.
	if s.rep.failures != 0 {
		return s.finishFailure()
	}
	s.rep.printTable(s.rep.out)
	s.rep.pass("live smoke completed; artifacts kept at %s", s.outDir)
	return 0
}

func (s *smoke) finishFailure() int {
	s.rep.printTable(s.rep.errw)
	summary, err := s.rep.writeFailureSummary(s.outDir, s.stderrs)
	if err == nil {
		fmt.Fprintf(s.rep.errw, "[INFO] failure summary: %s\n\n", summary)
		// #nosec G304 -- summary is a path this tool just wrote under its own
		// output dir, not external input.
		if body, rerr := os.ReadFile(summary); rerr == nil {
			_, _ = s.rep.errw.Write(body)
		}
	}
	fmt.Fprintf(s.rep.errw, "[FAIL] live smoke completed with %d failure(s); artifacts kept at %s\n", s.rep.failures, s.outDir)
	return 1
}

func (s *smoke) setupOutDir() int {
	if s.opts.OutDir == "" {
		dir, err := os.MkdirTemp("", "zscalerctl-live-smoke.")
		if err != nil {
			return s.usage("could not create temp dir: %v", err)
		}
		s.outDir = dir
	} else {
		if entries, err := os.ReadDir(s.opts.OutDir); err == nil && len(entries) > 0 {
			return s.usage("output directory already exists and is not empty: %s", s.opts.OutDir)
		}
		if err := os.MkdirAll(s.opts.OutDir, 0o700); err != nil {
			return s.usage("could not create output directory: %v", err)
		}
		s.outDir = s.opts.OutDir
	}
	// #nosec G302 -- this is a directory; 0700 (owner-only) is the intended,
	// tightest mode for it.
	_ = os.Chmod(s.outDir, 0o700)
	return 0
}

// loadResources runs schema list, parses it, and selects the resource set.
func (s *smoke) loadResources() bool {
	stdout, stderr, code := s.runner.Run("--format", "json", "schema", "list")
	if code != 0 {
		s.captureStderr("schema.stderr", stderr)
		s.rep.fail("schema list command failed")
		s.rep.record("schema", "list", "FAIL", "-", "command failed")
		return false
	}
	s.rep.pass("schema list command completed")

	specs, err := parseSchema(stdout)
	if err != nil {
		s.rep.fail("schema list is not a JSON array: %v", err)
		s.rep.record("schema", "list", "FAIL", "-", "invalid JSON")
		return false
	}
	s.specs = specs
	s.rep.pass("schema list returned a JSON array")

	all := catalogReadResources(specs)
	var defaults []string
	for _, q := range all {
		if resourceProduct(q) == "zia" {
			defaults = append(defaults, q)
		}
	}
	if len(defaults) == 0 {
		s.rep.fail("schema list contains no default ZIA read resources")
		return false
	}

	if len(s.requested) == 0 {
		s.resources = defaults
	} else {
		seen := map[string]bool{}
		for _, req := range s.requested {
			if !containsString(all, req) {
				s.rep.fail("requested resource is not a read resource: %s", req)
				return false
			}
			if !seen[req] {
				seen[req] = true
				s.resources = append(s.resources, req)
			}
		}
	}

	s.rep.pass("schema list found %d catalog read resource(s)", len(all))
	s.rep.pass("live smoke selected %d resource(s): %s", len(s.resources), strings.Join(s.resources, " "))
	s.rep.record("schema", "list", "PASS", fmt.Sprintf("%d", len(all)), fmt.Sprintf("selected %d resources", len(s.resources)))
	return true
}

func (s *smoke) validateResource(qualified string) {
	product := resourceProduct(qualified)
	name := resourceName(qualified)
	artifact := strings.ReplaceAll(qualified, "/", "-")
	spec := findSpec(s.specs, product, name)
	operation := ""
	if spec != nil {
		operation = spec.readOperation()
	}
	if operation == "" {
		s.rep.fail("schema list does not expose a supported read operation for %s", qualified)
		s.rep.record(qualified, "schema", "FAIL", "-", "no supported read operation")
		return
	}

	start := s.rep.failures
	stdout, stderr, code := s.runner.Run("--format", "json", product, name, operation)
	if code != 0 {
		s.captureStderr(artifact+".stderr", stderr)
		s.rep.fail("%s %s %s command failed", product, name, operation)
		s.rep.record(qualified, operation, "FAIL", "-", "command failed")
		return
	}
	s.rep.pass("%s %s %s command completed", product, name, operation)

	records := "-"
	data, perr := decodeJSON(stdout)
	label := fmt.Sprintf("%s %s %s", product, name, operation)
	if operation == "show" {
		if perr == nil && s.validateObject(label, data) {
			records = "1"
			s.listCounts[artifact] = 1
			s.runFieldChecks(label, product, name, data)
		} else if perr != nil {
			s.rep.fail("%s is not a JSON object: %v", label, perr)
		}
	} else {
		if perr == nil && s.validateArray(label, data) {
			n := arrayLen(data)
			records = fmt.Sprintf("%d", n)
			s.listCounts[artifact] = n
			s.runFieldChecks(label, product, name, data)
		} else if perr != nil {
			s.rep.fail("%s is not a JSON array: %v", label, perr)
		}
	}
	s.rep.recordFromFailures(qualified, operation, start, records, "", "see captured artifacts")
}

func (s *smoke) runFieldChecks(label, product, name string, data any) {
	if denied := findDeniedKeys(name, data); len(denied) > 0 {
		s.rep.fail("%s contains denied field key(s): %s", label, strings.Join(denied, " "))
	} else {
		s.rep.pass("%s contains no denied field keys", label)
	}
	if unexpected := findNonCatalogKeys(product, name, data, s.specs); len(unexpected) > 0 {
		s.rep.fail("%s contains non-catalog field key(s): %s", label, strings.Join(unexpected, " "))
	} else {
		s.rep.pass("%s contains only catalog-allowed top-level fields", label)
	}
	if paths := redactionMarkerPaths(data); len(paths) > 0 {
		s.rep.info("%s redaction markers at: %s", label, strings.Join(paths, " "))
	} else {
		s.rep.pass("%s has no redaction markers", label)
	}
}

func (s *smoke) validateArray(label string, data any) bool {
	arr, ok := data.([]any)
	if !ok {
		s.rep.fail("%s is not a JSON array", label)
		return false
	}
	s.rep.pass("%s returned a JSON array", label)
	if len(arr) == 0 {
		if s.opts.RequireNonempty {
			s.rep.fail("%s returned 0 records", label)
			return false
		}
		s.rep.info("%s returned 0 records", label)
	} else {
		s.rep.pass("%s returned %d records", label, len(arr))
	}
	return true
}

func (s *smoke) validateObject(label string, data any) bool {
	obj, ok := data.(map[string]any)
	if !ok {
		s.rep.fail("%s is not a JSON object", label)
		return false
	}
	s.rep.pass("%s returned a JSON object", label)
	if len(obj) == 0 {
		s.rep.fail("%s returned an empty JSON object", label)
		return false
	}
	s.rep.pass("%s returned 1 record", label)
	return true
}

func (s *smoke) selectedProducts() []string {
	seen := map[string]bool{}
	var out []string
	for _, q := range s.resources {
		p := resourceProduct(q)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func (s *smoke) selectedHasProduct(want string) bool {
	for _, q := range s.resources {
		if resourceProduct(q) == want {
			return true
		}
	}
	return false
}

func (s *smoke) selectedHasNonZIA() bool {
	for _, q := range s.resources {
		if resourceProduct(q) != "zia" {
			return true
		}
	}
	return false
}

func (s *smoke) captureStderr(name string, body []byte) {
	s.stderrs = append(s.stderrs, namedStderr{name: name, body: body})
}

func decodeJSON(body []byte) (any, error) {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func arrayLen(data any) int {
	if arr, ok := data.([]any); ok {
		return len(arr)
	}
	return 0
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

const defaultManifest = "live-smoke.manifest"

func (s *smoke) loadManifest(path string) ([]string, error) {
	// #nosec G304 -- path is the operator-supplied --manifest/default manifest
	// path for this dev tool, not external request input.
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("live smoke manifest not found: %s", path)
	}
	return parseManifest(body)
}

// shouldUseDefaultManifest mirrors the shell auto-discovery: only on a feature
// branch where live-smoke.manifest exists and differs from the base.
func (s *smoke) shouldUseDefaultManifest() bool {
	if s.opts.NoManifest || len(s.requested) > 0 {
		return false
	}
	if _, err := os.Stat(defaultManifest); err != nil {
		return false
	}
	branch := gitOutput("branch", "--show-current")
	switch strings.TrimSpace(branch) {
	case "", "main", "master":
		return false
	}
	return manifestChangedFromBase()
}

// gitOutput runs a fixed git query (for default-manifest branch auto-discovery)
// and returns its stdout, or "" on error.
func gitOutput(args ...string) string {
	// #nosec G204 -- fixed git subcommands for branch/diff discovery; args are
	// constants from this file, not external input.
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func manifestChangedFromBase() bool {
	base := osEnv("LIVE_SMOKE_MANIFEST_BASE")
	if base == "" {
		base = "origin/main"
	}
	if gitFails("rev-parse", "--is-inside-work-tree") {
		return true
	}
	if gitFails("rev-parse", "--verify", "-q", base) {
		return true
	}
	if gitFails("diff", "--quiet", base+"...HEAD", "--", defaultManifest) {
		return true
	}
	if gitFails("diff", "--quiet", "--", defaultManifest) {
		return true
	}
	others := strings.TrimSpace(gitOutput("ls-files", "--others", "--exclude-standard", "--", defaultManifest))
	return others != ""
}

// gitFails runs a fixed git command and reports whether it exited non-zero.
func gitFails(args ...string) bool {
	// #nosec G204 -- fixed git subcommands for default-manifest discovery; the
	// only variable is a branch ref read from config/env, not request input.
	return exec.Command("git", args...).Run() != nil
}

// fileMode returns the permission bits of path as a 3-digit octal string.
func fileMode(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%03o", info.Mode().Perm()), nil
}

func (s *smoke) validateFileMode(label, path, want string) {
	got, err := fileMode(path)
	if err != nil {
		s.rep.fail("%s missing: %s", label, path)
		return
	}
	if got != want {
		s.rep.fail("%s mode = %s, want %s: %s", label, got, want, path)
		return
	}
	s.rep.pass("%s mode is %s", label, want)
}

var _ = filepath.Join
