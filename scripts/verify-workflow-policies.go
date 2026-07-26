//go:build ignore

// verify-workflow-policies validates the static GitHub Actions policy shared by
// the action-pin, Go-toolchain, and Node-toolchain shell gates. It is
// deliberately a development helper, not part of the zscalerctl binary.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	modeActions       = "actions"
	modeGo            = "setup-go"
	modeNode          = "setup-node"
	reviewedCheckout  = "actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd"
	reviewedSetupGo   = "actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c"
	reviewedSetupNode = "actions/setup-node@48b55a011bda9f5d6aeb4c2d9c7362e8dae4041e"
)

type fileKind uint8

const (
	workflowKind fileKind = iota
	actionKind
)

var (
	fullSHARef = regexp.MustCompile(`^[^@[:space:]]+@[0-9a-fA-F]{40}$`)
	commentVer = regexp.MustCompile(`^v?[0-9][A-Za-z0-9._-]*`)
	lineInErr  = regexp.MustCompile(`line ([0-9]+)`) // yaml.v3 parser errors
)

var retiredRuntimeActions = map[string]struct{}{
	"gitleaks/gitleaks-action@ff98106e4c7b2bc287b24eaf42907196329070c7": {},
}

type verifier struct {
	mode                string
	goMinimum           string
	nodeFile            string
	requiredRun         string
	requiredRunJob      string
	requiredRunIf       string
	requiredDepJob      string
	requiredDepIf       string
	requiredDepRun      string
	requiredDepNeed     map[string]struct{}
	requiredDepAll      bool
	requiredConsumer    string
	requiredConsumerJob string
	requiredJobs        map[string]struct{}
	requiredRunFile     string
	rootReal            string

	active  map[string]bool
	visited map[visitKey]bool
	stack   []string
	errors  []string

	setupGoCount          int
	setupNodeCount        int
	nodeConsumerCount     int
	requiredConsumerCount int
	requiredRunCount      int
}

type mapEntry struct {
	key   *yaml.Node
	value *yaml.Node
}

type visitKey struct {
	path string
	kind fileKind
}

type nodeJobState struct {
	setupReady          bool
	runtimeReady        bool
	runtimeCheckPending bool
	setupSeen           bool
	checkoutSeen        bool
	setupGoSeen         bool
	semgrepInstallSeen  bool
	unreviewedBeforeSet []*yaml.Node
}

func main() {
	mode := flag.String("mode", "", "policy to check: actions, setup-go, or setup-node")
	scanDir := flag.String("scan-dir", "", "directory containing workflow YAML files")
	repoRoot := flag.String("repo-root", "", "repository root used for local reference resolution")
	goMinimum := flag.String("go-minimum", "", "required literal setup-go version in setup-go mode")
	nodeFile := flag.String("node-version-file", "", "required repository-relative setup-node version file in setup-node mode")
	requiredRun := flag.String("required-run", "", "required literal run command in setup-node mode")
	requiredRunJob := flag.String("required-run-job", "", "workflow job that must contain --required-run in setup-node mode")
	requiredRunIf := flag.String("required-run-if", "", "required literal if condition for --required-run in setup-node mode")
	requiredDepJob := flag.String("required-dependent-job", "", "workflow job that must depend on --required-run-job in setup-node mode")
	requiredDepIf := flag.String("required-dependent-job-if", "", "required literal if condition for --required-dependent-job")
	requiredDepRun := flag.String("required-dependent-run", "", "required sole run command for --required-dependent-job")
	requiredDepNeeds := flag.String("required-dependent-needs", "", "comma-separated jobs that --required-dependent-job must need")
	requiredDepAll := flag.Bool("required-dependent-needs-all-jobs", false, "require --required-dependent-job to need every other workflow job exactly once")
	requiredConsumer := flag.String("required-consumer", "", "recognized Node consumer command required in --required-consumer-job")
	requiredConsumerJob := flag.String("required-consumer-job", "", "workflow job that must contain --required-consumer")
	requiredJobSet := flag.String("required-job-set", "", "comma-separated exact workflow job set in setup-node mode")
	flag.Parse()

	if *mode != modeActions && *mode != modeGo && *mode != modeNode {
		failUsage("--mode must be actions, setup-go, or setup-node")
	}
	if *scanDir == "" || *repoRoot == "" {
		failUsage("--scan-dir and --repo-root are required")
	}
	if *mode == modeGo && *goMinimum == "" {
		failUsage("--go-minimum is required in setup-go mode")
	}
	if *mode == modeNode && *nodeFile == "" {
		failUsage("--node-version-file is required in setup-node mode")
	}
	if *mode != modeNode && (*requiredRun != "" || *requiredRunJob != "" || *requiredRunIf != "" || *requiredDepJob != "" || *requiredDepIf != "" || *requiredDepRun != "" || *requiredDepNeeds != "" || *requiredDepAll || *requiredConsumer != "" || *requiredConsumerJob != "" || *requiredJobSet != "") {
		failUsage("required-run and required-dependent-job flags are valid only in setup-node mode")
	}
	if (*requiredRun == "") != (*requiredRunJob == "") {
		failUsage("--required-run and --required-run-job must be provided together")
	}
	if *requiredRunIf != "" && *requiredRun == "" {
		failUsage("--required-run-if requires --required-run and --required-run-job")
	}
	if *requiredDepJob != "" && *requiredRun == "" {
		failUsage("--required-dependent-job requires --required-run and --required-run-job")
	}
	if *requiredDepIf != "" && *requiredDepJob == "" {
		failUsage("--required-dependent-job-if requires --required-dependent-job")
	}
	if *requiredDepRun != "" && *requiredDepJob == "" {
		failUsage("--required-dependent-run requires --required-dependent-job")
	}
	if *requiredDepNeeds != "" && *requiredDepJob == "" {
		failUsage("--required-dependent-needs requires --required-dependent-job")
	}
	if *requiredDepAll && *requiredDepJob == "" {
		failUsage("--required-dependent-needs-all-jobs requires --required-dependent-job")
	}
	if *requiredDepAll && *requiredDepNeeds != "" {
		failUsage("--required-dependent-needs-all-jobs and --required-dependent-needs are mutually exclusive")
	}
	if (*requiredConsumer == "") != (*requiredConsumerJob == "") {
		failUsage("--required-consumer and --required-consumer-job must be provided together")
	}
	if *requiredConsumer != "" && !isNodeConsumerCommand(*requiredConsumer) {
		failUsage("--required-consumer must name a recognized Node consumer command")
	}
	if *mode == modeNode && *goMinimum == "" {
		failUsage("--go-minimum is required in setup-node mode")
	}

	v, err := newVerifier(*mode, *repoRoot, *goMinimum, *nodeFile, *requiredRun, *requiredRunJob, *requiredRunIf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify-workflow-policies: %v\n", err)
		os.Exit(1)
	}
	v.requiredDepJob = *requiredDepJob
	v.requiredDepIf = *requiredDepIf
	v.requiredDepRun = *requiredDepRun
	v.requiredDepAll = *requiredDepAll
	v.requiredConsumer = *requiredConsumer
	v.requiredConsumerJob = *requiredConsumerJob
	if *requiredDepNeeds == "" && *requiredDepJob != "" && !*requiredDepAll {
		v.requiredDepNeed = map[string]struct{}{*requiredRunJob: {}}
	} else if *requiredDepNeeds != "" {
		v.requiredDepNeed, err = parseRequiredJobSet("--required-dependent-needs", *requiredDepNeeds)
		if err != nil {
			failUsage(err.Error())
		}
	}
	if *requiredJobSet != "" {
		v.requiredJobs, err = parseRequiredJobSet("--required-job-set", *requiredJobSet)
		if err != nil {
			failUsage(err.Error())
		}
	}

	files, err := yamlFiles(*scanDir, v.rootReal)
	if err != nil {
		v.addError(*scanDir, 1, 1, "%v", err)
	} else {
		if v.requiredRun != "" {
			if len(files) != 1 {
				v.addError(*scanDir, 1, 1, "--required-run requires --scan-dir to identify exactly one workflow file")
			} else {
				v.requiredRunFile = filepath.Clean(files[0])
			}
		}
		for _, file := range files {
			v.scanFile(file, nil, workflowKind)
		}
	}
	if *mode == modeActions {
		files, err := localActionFiles(v.rootReal)
		if err != nil {
			v.addError(v.rootReal, 1, 1, "%v", err)
		} else {
			for _, file := range files {
				v.scanFile(file, nil, actionKind)
			}
		}
	}

	if *mode == modeGo && v.setupGoCount == 0 && len(v.errors) == 0 {
		v.addError(*scanDir, 1, 1, "no setup-go steps found")
	}
	if *mode == modeNode {
		if v.setupNodeCount == 0 {
			v.addError(*scanDir, 1, 1, "no direct setup-node steps found")
		}
		if v.nodeConsumerCount == 0 {
			v.addError(*scanDir, 1, 1, "no recognized direct Node consumer steps found")
		}
		if v.requiredConsumer != "" && v.requiredConsumerCount != 1 {
			v.addError(
				*scanDir,
				1,
				1,
				"required Node consumer %q must appear exactly once in workflow job %q; found %d",
				v.requiredConsumer,
				v.requiredConsumerJob,
				v.requiredConsumerCount,
			)
		}
		if v.requiredRun != "" && v.requiredRunCount == 0 {
			v.addError(
				*scanDir,
				1,
				1,
				"required run %q was not found with the mandated condition in workflow job %q",
				v.requiredRun,
				v.requiredRunJob,
			)
		}
	}

	for _, message := range v.errors {
		fmt.Fprintln(os.Stderr, message)
	}
	if len(v.errors) != 0 {
		os.Exit(1)
	}
}

func failUsage(message string) {
	fmt.Fprintf(os.Stderr, "verify-workflow-policies: %s\n", message)
	fmt.Fprintln(os.Stderr, "usage: go run ./scripts/verify-workflow-policies.go --mode actions|setup-go|setup-node --scan-dir DIR --repo-root DIR [--go-minimum VERSION] [--node-version-file PATH] [--required-run COMMAND --required-run-job JOB [--required-run-if CONDITION] [--required-dependent-job JOB [--required-dependent-job-if CONDITION] [--required-dependent-run COMMAND] [--required-dependent-needs JOB,... | --required-dependent-needs-all-jobs]]] [--required-consumer COMMAND --required-consumer-job JOB] [--required-job-set JOB,...]")
	os.Exit(2)
}

func parseRequiredJobSet(flagName, value string) (map[string]struct{}, error) {
	jobs := make(map[string]struct{})
	for _, name := range strings.Split(value, ",") {
		if name == "" || strings.TrimSpace(name) != name {
			return nil, fmt.Errorf("%s must contain non-empty job names without surrounding whitespace", flagName)
		}
		if _, duplicate := jobs[name]; duplicate {
			return nil, fmt.Errorf("%s contains duplicate job %q", flagName, name)
		}
		jobs[name] = struct{}{}
	}
	return jobs, nil
}

func newVerifier(mode, repoRoot, goMinimum, nodeFile, requiredRun, requiredRunJob, requiredRunIf string) (*verifier, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root %q: %w", repoRoot, err)
	}
	rootInfo, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("repository root %q: %w", repoRoot, err)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("repository root %q is not a directory", repoRoot)
	}
	rootReal, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root %q: %w", repoRoot, err)
	}
	return &verifier{
		mode:           mode,
		goMinimum:      goMinimum,
		nodeFile:       nodeFile,
		requiredRun:    requiredRun,
		requiredRunJob: requiredRunJob,
		requiredRunIf:  requiredRunIf,
		rootReal:       rootReal,
		active:         map[string]bool{},
		visited:        map[visitKey]bool{},
	}, nil
}

func yamlFiles(scanDir, rootReal string) ([]string, error) {
	absDir, err := filepath.Abs(scanDir)
	if err != nil {
		return nil, fmt.Errorf("resolve scan directory %q: %w", scanDir, err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return nil, fmt.Errorf("scan directory %q: %w", scanDir, err)
	}

	if !info.IsDir() {
		if !isYAMLFile(absDir) {
			return nil, fmt.Errorf("scan path %q is not a YAML file or directory", scanDir)
		}
		if err := ensureInside(rootReal, absDir); err != nil {
			return nil, err
		}
		return []string{absDir}, nil
	}

	realDir, err := filepath.EvalSymlinks(absDir)
	if err != nil {
		return nil, fmt.Errorf("resolve scan directory %q: %w", scanDir, err)
	}
	if err := ensureInside(rootReal, realDir); err != nil {
		return nil, fmt.Errorf("scan directory %q: %w", scanDir, err)
	}

	var files []string
	err = filepath.WalkDir(absDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if isYAMLFile(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk scan directory %q: %w", scanDir, err)
	}
	sort.Strings(files)
	return files, nil
}

func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yml" || ext == ".yaml"
}

func localActionFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == "action.yml" || entry.Name() == "action.yaml" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover local action metadata: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func (v *verifier) scanFile(path string, via *yaml.Node, kind fileKind) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		v.addNodeError(path, via, "resolve YAML path: %v", err)
		return
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		v.addNodeError(absPath, via, "resolve YAML path: %v", err)
		return
	}
	if err := ensureInside(v.rootReal, realPath); err != nil {
		v.addNodeError(absPath, via, "%v", err)
		return
	}
	key := visitKey{path: realPath, kind: kind}
	if v.active[realPath] {
		v.addNodeError(absPath, via, "local reference dependency cycle: %s", v.cycleDescription(realPath))
		return
	}
	if v.visited[key] {
		return
	}

	data, err := os.ReadFile(realPath)
	if err != nil {
		v.addNodeError(absPath, via, "read YAML: %v", err)
		v.visited[key] = true
		return
	}

	v.active[realPath] = true
	v.stack = append(v.stack, realPath)
	defer func() {
		delete(v.active, realPath)
		v.stack = v.stack[:len(v.stack)-1]
		v.visited[key] = true
	}()

	root, ok := decodeYAML(data)
	if !ok {
		v.addErrorWithYAMLLine(absPath, root.err, "malformed YAML: %v")
		return
	}
	if root.node == nil || root.node.Kind != yaml.MappingNode {
		line, column := nodePosition(root.node)
		v.addError(absPath, line, column, "unsupported YAML root: expected a mapping")
		return
	}

	switch kind {
	case workflowKind:
		v.scanWorkflow(absPath, root.node)
	case actionKind:
		v.scanLocalAction(absPath, root.node)
	}
}

type decodedYAML struct {
	node *yaml.Node
	err  error
}

func decodeYAML(data []byte) (decodedYAML, bool) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return decodedYAML{err: err}, false
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return decodedYAML{node: &document}, true
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return decodedYAML{err: fmt.Errorf("multiple YAML documents are unsupported")}, false
		}
		return decodedYAML{err: err}, false
	}
	return decodedYAML{node: document.Content[0]}, true
}

func (v *verifier) scanWorkflow(file string, root *yaml.Node) {
	entries, ok := v.executableEntries(file, root, "workflow")
	if !ok {
		return
	}
	if v.mode == modeNode {
		if defaults, found := entryValue(entries, "defaults"); found {
			v.addNodeError(file, defaults, "Node policy workflows must not override run defaults")
		}
		if environment, found := entryValue(entries, "env"); found {
			v.addNodeError(file, environment, "Node policy workflows must not define workflow-level environment overrides")
		}
		if len(v.requiredJobs) != 0 {
			v.checkReleaseWorkflowPermissions(file, entries)
		}
	}
	jobs, found := entryValue(entries, "jobs")
	if !found {
		v.addNodeError(file, root, "workflow is missing jobs mapping")
		return
	}
	jobEntries, ok := v.executableEntries(file, jobs, "workflow jobs")
	if !ok {
		return
	}
	if v.mode == modeNode && len(v.requiredJobs) != 0 {
		v.checkExactJobSet(file, jobEntries)
	}
	if v.mode == modeNode && v.requiredDepJob != "" {
		v.checkRequiredDependentJob(file, jobEntries)
	}
	for _, job := range jobEntries {
		if job.value.Kind != yaml.MappingNode {
			v.addNodeError(file, job.value, "workflow job %q must be a YAML mapping", job.key.Value)
			continue
		}
		v.scanJob(file, job.key.Value, job.value)
	}
}

func (v *verifier) checkReleaseWorkflowPermissions(file string, entries []mapEntry) {
	permissions, found := entryValue(entries, "permissions")
	if !found {
		v.addNodeError(file, nil, "release workflow must set exact top-level contents: read permissions")
		return
	}
	permissionEntries, ok := v.executableEntries(file, permissions, "release workflow permissions")
	if !ok {
		return
	}
	if len(permissionEntries) != 1 || permissionEntries[0].key.Value != "contents" ||
		permissionEntries[0].value.Kind != yaml.ScalarNode || permissionEntries[0].value.Tag != "!!str" ||
		permissionEntries[0].value.Value != "read" {
		v.addNodeError(file, permissions, "release workflow permissions must contain only literal contents: read")
	}
}

func (v *verifier) checkExactJobSet(file string, jobs []mapEntry) {
	seen := make(map[string]struct{}, len(jobs))
	for _, job := range jobs {
		name := job.key.Value
		seen[name] = struct{}{}
		if _, allowed := v.requiredJobs[name]; !allowed {
			v.addNodeError(file, job.key, "workflow job %q is not in the exact reviewed job set", name)
		}
	}
	for name := range v.requiredJobs {
		if _, found := seen[name]; !found {
			v.addNodeError(file, nil, "required workflow job %q was not found", name)
		}
	}
}

func (v *verifier) checkRequiredDependentJob(file string, jobs []mapEntry) {
	var dependent *yaml.Node
	for _, job := range jobs {
		if job.key.Value == v.requiredDepJob {
			dependent = job.value
			break
		}
	}
	if dependent == nil {
		v.addNodeError(file, nil, "required dependent job %q was not found", v.requiredDepJob)
		return
	}
	entries, ok := v.executableEntries(file, dependent, "required dependent job")
	if !ok {
		return
	}
	needs, found := entryValue(entries, "needs")
	if !found {
		v.addNodeError(file, needs, "required dependent job %q must declare reviewed dependencies", v.requiredDepJob)
	} else {
		expected := v.requiredDepNeed
		if v.requiredDepAll {
			expected = make(map[string]struct{}, len(jobs)-1)
			for _, job := range jobs {
				if job.key.Value != v.requiredDepJob {
					expected[job.key.Value] = struct{}{}
				}
			}
		}
		actual, valid := literalNeedsSet(needs)
		if !valid {
			v.addNodeError(file, needs, "required dependent job %q must declare literal, unique reviewed dependencies", v.requiredDepJob)
		} else {
			for required := range expected {
				if _, included := actual[required]; !included {
					v.addNodeError(file, needs, "required dependent job %q must need reviewed job %q", v.requiredDepJob, required)
				}
			}
			for dependency := range actual {
				if _, reviewed := expected[dependency]; !reviewed {
					v.addNodeError(file, needs, "required dependent job %q must not need unreviewed job %q", v.requiredDepJob, dependency)
				}
			}
		}
	}
	condition, conditional := entryValue(entries, "if")
	if v.requiredDepIf == "" {
		if conditional {
			v.addNodeError(file, condition, "required dependent job %q must be unconditional", v.requiredDepJob)
		}
	} else if !conditional || condition.Kind != yaml.ScalarNode || condition.Tag != "!!str" || condition.Value != v.requiredDepIf {
		v.addNodeError(file, condition, "required dependent job %q must use the literal condition %q", v.requiredDepJob, v.requiredDepIf)
	}
	if v.requiredDepRun != "" {
		v.checkRequiredDependentRun(file, entries)
	}
	if len(v.requiredJobs) != 0 {
		v.checkReleasePublisherCheckout(file, entries)
	}
}

func (v *verifier) checkReleasePublisherCheckout(file string, entries []mapEntry) {
	steps, found := entryValue(entries, "steps")
	if !found || steps.Kind != yaml.SequenceNode {
		v.addNodeError(file, steps, "release publisher must define a literal steps sequence")
		return
	}
	checkoutCount := 0
	for index, step := range steps.Content {
		stepEntries, ok := v.executableEntries(file, step, "release publisher step")
		if !ok {
			continue
		}
		uses, found := entryValue(stepEntries, "uses")
		if !found {
			continue
		}
		ref, ok := v.literalReference(file, uses, "release publisher step uses")
		if !ok || !strings.EqualFold(actionName(ref), "actions/checkout") {
			continue
		}
		checkoutCount++
		if actionName(ref) != "actions/checkout" {
			v.addNodeError(file, uses, "release publisher checkout must use the canonical lowercase action name actions/checkout")
		}
		if index != 0 {
			v.addNodeError(file, uses, "release publisher checkout must be the first step and the only checkout action")
		}
		v.checkNodeBootstrapAction(file, "release", stepEntries, ref)
	}
	if checkoutCount != 1 {
		v.addNodeError(file, steps, "release publisher must contain exactly one canonical checkout step, found %d", checkoutCount)
	}
}

func (v *verifier) checkRequiredDependentRun(file string, entries []mapEntry) {
	for _, entry := range entries {
		switch entry.key.Value {
		case "if", "name", "needs", "runs-on", "steps":
		default:
			v.addNodeError(file, entry.key, "required dependent job %q key %q is not allowed", v.requiredDepJob, entry.key.Value)
		}
	}
	runner, found := entryValue(entries, "runs-on")
	if !found || runner.Kind != yaml.ScalarNode || runner.Tag != "!!str" || runner.Value != "ubuntu-latest" {
		v.addNodeError(file, runner, "required dependent job %q must use the literal runner ubuntu-latest", v.requiredDepJob)
	}
	steps, found := entryValue(entries, "steps")
	if !found || steps.Kind != yaml.SequenceNode || len(steps.Content) != 2 {
		v.addNodeError(file, steps, "required dependent job %q must contain exactly the reviewed checkout and result-check steps", v.requiredDepJob)
		return
	}
	checkoutEntries, ok := v.executableEntries(file, steps.Content[0], "required dependent job checkout step")
	if !ok {
		return
	}
	checkoutUses, found := entryValue(checkoutEntries, "uses")
	if !found {
		v.addNodeError(file, steps.Content[0], "required dependent job %q must begin with canonical checkout", v.requiredDepJob)
	} else if ref, valid := v.literalReference(file, checkoutUses, "required dependent checkout uses"); !valid || actionName(ref) != "actions/checkout" {
		v.addNodeError(file, checkoutUses, "required dependent job %q must begin with canonical actions/checkout", v.requiredDepJob)
	} else {
		v.checkNodeBootstrapAction(file, v.requiredDepJob, checkoutEntries, ref)
	}
	stepEntries, ok := v.executableEntries(file, steps.Content[1], "required dependent job result-check step")
	if !ok {
		return
	}
	for _, entry := range stepEntries {
		switch entry.key.Value {
		case "name", "run":
		default:
			v.addNodeError(file, entry.key, "required dependent job %q step key %q is not allowed", v.requiredDepJob, entry.key.Value)
		}
	}
	run, found := entryValue(stepEntries, "run")
	if !found || run.Kind != yaml.ScalarNode || run.Tag != "!!str" || run.Value != v.requiredDepRun {
		v.addNodeError(file, run, "required dependent job %q must run exactly %q", v.requiredDepJob, v.requiredDepRun)
	}
}

func literalNeedsSet(node *yaml.Node) (map[string]struct{}, bool) {
	if node == nil {
		return nil, false
	}
	result := make(map[string]struct{})
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Tag != "!!str" || node.Value == "" {
			return nil, false
		}
		result[node.Value] = struct{}{}
		return result, true
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode || item.Tag != "!!str" || item.Value == "" {
				return nil, false
			}
			if _, duplicate := result[item.Value]; duplicate {
				return nil, false
			}
			result[item.Value] = struct{}{}
		}
		return result, len(result) != 0
	}
	return nil, false
}

func (v *verifier) scanJob(file, jobName string, job *yaml.Node) {
	entries, ok := v.executableEntries(file, job, "workflow job")
	if !ok {
		return
	}
	uses, hasUses := entryValue(entries, "uses")
	steps, hasSteps := entryValue(entries, "steps")
	if hasUses && hasSteps {
		v.addNodeError(file, job, "workflow job cannot define both uses and steps")
		return
	}
	if hasUses {
		v.inspectJobUses(file, job, uses)
		return
	}
	if hasSteps {
		if v.mode == modeNode {
			if relevant := v.scanNodeSteps(file, jobName, steps); relevant {
				v.checkNodePolicyJob(file, jobName, entries)
				if condition, conditional := entryValue(entries, "if"); conditional {
					v.addNodeError(file, condition, "job %q containing Node policy steps must be unconditional", jobName)
				}
				if continuation, found := entryValue(entries, "continue-on-error"); found {
					v.addNodeError(file, continuation, "job %q containing Node policy steps must not set continue-on-error", jobName)
				}
				if defaults, found := entryValue(entries, "defaults"); found {
					v.addNodeError(file, defaults, "job %q containing Node policy steps must not override run defaults", jobName)
				}
				if environment, found := entryValue(entries, "env"); found {
					v.addNodeError(file, environment, "job %q containing Node policy steps must not define environment overrides", jobName)
				}
			}
			return
		}
		v.scanSteps(file, steps)
		return
	}
	v.addNodeError(file, job, "workflow job must define uses or steps")
}

func (v *verifier) scanNodeSteps(file, jobName string, steps *yaml.Node) bool {
	if steps.Kind != yaml.SequenceNode {
		v.addNodeError(file, steps, "unsupported or dynamic steps structure; steps must be a YAML sequence")
		return false
	}

	policyJob := v.nodePolicyJobCandidate(jobName, steps)
	state := nodeJobState{}
	relevant := false
	for _, step := range steps.Content {
		if step.Kind != yaml.MappingNode {
			v.addNodeError(file, step, "unsupported workflow step structure; each step must be a YAML mapping")
			if state.runtimeCheckPending {
				state.setupReady = false
				state.runtimeCheckPending = false
			}
			state.runtimeReady = false
			continue
		}
		var stepRelevant bool
		state, stepRelevant = v.scanNodeStep(file, jobName, step, state, policyJob)
		relevant = relevant || stepRelevant
	}
	if relevant {
		for _, unreviewed := range state.unreviewedBeforeSet {
			v.addNodeError(file, unreviewed, "unreviewed step is not allowed in a Node policy job")
		}
		if policyJob {
			v.checkNodeBootstrapComplete(file, jobName, state)
		}
	}
	return relevant
}

func (v *verifier) nodePolicyJobCandidate(jobName string, steps *yaml.Node) bool {
	if jobName == v.requiredRunJob {
		return true
	}
	for _, step := range steps.Content {
		if step.Kind != yaml.MappingNode {
			continue
		}
		for index := 0; index+1 < len(step.Content); index += 2 {
			key, value := step.Content[index], step.Content[index+1]
			if key.Kind != yaml.ScalarNode || value.Kind != yaml.ScalarNode {
				continue
			}
			switch key.Value {
			case "uses":
				if actionName(value.Value) == "actions/setup-node" {
					return true
				}
			case "run":
				command := strings.TrimSpace(value.Value)
				if command == v.requiredRun || command == "/bin/bash scripts/verify-active-node-toolchain.sh" || isNodeConsumerCommand(command) {
					return true
				}
			}
		}
	}
	return false
}

func (v *verifier) scanNodeStep(file, jobName string, step *yaml.Node, state nodeJobState, policyJob bool) (nodeJobState, bool) {
	entries, ok := v.executableEntries(file, step, "workflow step")
	if !ok {
		if state.runtimeCheckPending {
			state.setupReady = false
			state.runtimeCheckPending = false
		}
		state.runtimeReady = false
		return state, false
	}
	uses, hasUses := entryValue(entries, "uses")
	run, hasRun := entryValue(entries, "run")
	if hasUses && hasRun {
		v.addNodeError(file, step, "workflow step cannot define both uses and run")
		if state.runtimeCheckPending {
			state.setupReady = false
			state.runtimeCheckPending = false
		}
		state.runtimeReady = false
		return state, false
	}

	if hasUses {
		ref, ok := v.literalReference(file, uses, "step uses")
		if !ok {
			state.runtimeReady = false
			return state, false
		}
		action := actionName(ref)
		if action != "actions/setup-node" {
			if !policyJob {
				return state, false
			}
			if !state.setupSeen {
				switch action {
				case "actions/checkout":
					if state.checkoutSeen || state.setupGoSeen {
						v.addNodeError(file, uses, "actions/checkout must appear exactly once and before setup-go in a Node policy job")
					}
					state.checkoutSeen = true
					v.checkNodeBootstrapAction(file, jobName, entries, ref)
				case "actions/setup-go":
					if !state.checkoutSeen || state.setupGoSeen {
						v.addNodeError(file, uses, "actions/setup-go must appear exactly once after checkout in a Node policy job")
					}
					state.setupGoSeen = true
					v.checkNodeBootstrapAction(file, jobName, entries, ref)
				default:
					state.unreviewedBeforeSet = append(state.unreviewedBeforeSet, uses)
				}
				return state, false
			}
			v.addNodeError(file, uses, "unreviewed action %q is not allowed after setup-node in a Node policy job", ref)
			state.setupReady = false
			state.runtimeCheckPending = false
			state.runtimeReady = false
			return state, false
		}

		for _, unreviewed := range state.unreviewedBeforeSet {
			v.addNodeError(file, unreviewed, "unreviewed step may not precede setup-node in a Node policy job")
		}
		state.unreviewedBeforeSet = nil
		duplicateSetup := state.setupSeen
		if duplicateSetup {
			v.addNodeError(file, uses, "setup-node may appear only once in a Node policy job")
		}
		state.setupSeen = true
		state.runtimeReady = false

		v.setupNodeCount++
		valid := !duplicateSetup
		if ref != reviewedSetupNode {
			v.addNodeError(file, uses, "Node policy setup-node must use reviewed action ref %s", reviewedSetupNode)
			valid = false
		}
		if !v.checkSetupNodeStepKeys(file, entries) {
			valid = false
		}
		if condition, conditional := entryValue(entries, "if"); conditional {
			v.addNodeError(file, condition, "setup-node must be unconditional; remove the step-level if condition")
			valid = false
		}
		if continuation, found := entryValue(entries, "continue-on-error"); found {
			v.addNodeError(file, continuation, "setup-node must not set continue-on-error")
			valid = false
		}
		if !v.checkSetupNode(file, entries, uses) {
			valid = false
		}
		state.setupReady = valid
		state.runtimeCheckPending = valid
		return state, true
	}

	if !hasRun {
		if state.runtimeCheckPending {
			state.setupReady = false
			state.runtimeCheckPending = false
		}
		state.runtimeReady = false
		return state, false
	}
	command, ok := v.literalRunCommand(file, run)
	if !ok {
		state.runtimeReady = false
		return state, false
	}

	consumer := isNodeConsumerCommand(command)
	runtimeCheck := command == "/bin/bash scripts/verify-active-node-toolchain.sh"
	required := filepath.Clean(file) == v.requiredRunFile && command == v.requiredRun
	relevant := consumer || runtimeCheck || required
	if policyJob && !state.setupSeen && !consumer && !runtimeCheck && !required {
		if jobName == "release-gate" && command == "python3 -m pip install --user --require-hashes -r .github/requirements/semgrep.txt" && !state.semgrepInstallSeen {
			state.semgrepInstallSeen = true
			v.checkNodeBootstrapRun(file, entries, command)
		} else {
			state.unreviewedBeforeSet = append(state.unreviewedBeforeSet, run)
		}
	}
	if policyJob && state.setupSeen && !consumer && !runtimeCheck {
		v.addNodeError(file, run, "unreviewed run %q is not allowed after setup-node in a Node policy job", command)
	}
	if state.runtimeCheckPending && !runtimeCheck {
		state.setupReady = false
		state.runtimeCheckPending = false
	}
	if state.runtimeReady && !consumer {
		state.runtimeReady = false
	}
	if relevant {
		v.checkRunExecutionContext(file, entries, command)
	}
	if runtimeCheck {
		valid := state.setupReady
		if !state.setupReady {
			v.addNodeError(file, run, "active Node verification must follow a valid direct setup-node step in the same job")
		}
		if !v.checkNodeRuntimeCondition(file, entries, command) {
			valid = false
		}
		state.runtimeCheckPending = false
		state.setupReady = false
		state.runtimeReady = valid
		return state, true
	}
	if consumer {
		v.nodeConsumerCount++
		if command == v.requiredConsumer && jobName == v.requiredConsumerJob {
			v.requiredConsumerCount++
		}
		if !state.setupSeen {
			v.addNodeError(
				file,
				run,
				"Node consumer %q must follow a valid unconditional direct setup-node step in the same job",
				command,
			)
		}
		if !state.runtimeReady {
			v.addNodeError(file, run, "Node consumer %q must immediately follow exact active Node verification", command)
		}
		v.checkNodeConsumerStep(file, entries, command)
		state.setupReady = false
		state.runtimeReady = false
	}

	if !required {
		return state, relevant
	}
	if jobName != v.requiredRunJob {
		v.addNodeError(file, run, "required run %q must be in workflow job %q", command, v.requiredRunJob)
		return state, relevant
	}
	condition, conditional := entryValue(entries, "if")
	if v.requiredRunIf == "" {
		if conditional {
			v.addNodeError(file, condition, "required run %q must be unconditional", command)
			return state, relevant
		}
	} else if !conditional || condition.Kind != yaml.ScalarNode || condition.Tag != "!!str" || condition.Value != v.requiredRunIf {
		v.addNodeError(file, condition, "required run %q must use the literal condition %q", command, v.requiredRunIf)
		return state, relevant
	}
	if continuation, found := entryValue(entries, "continue-on-error"); found && !consumer {
		v.addNodeError(file, continuation, "required run %q must not set continue-on-error", command)
		return state, relevant
	}
	v.requiredRunCount++
	return state, relevant
}

func isNodeConsumerCommand(command string) bool {
	switch command {
	case "/bin/bash scripts/verify-typescript-client.sh",
		"/usr/bin/make release-check":
		return true
	default:
		return false
	}
}

func (v *verifier) checkNodeConsumerStep(file string, entries []mapEntry, command string) {
	if continuation, found := entryValue(entries, "continue-on-error"); found {
		v.addNodeError(file, continuation, "Node consumer %q must not set continue-on-error", command)
	}

	if condition, conditional := entryValue(entries, "if"); conditional {
		v.addNodeError(file, condition, "Node consumer %q must be unconditional", command)
	}
}

func (v *verifier) checkNodeBootstrapAction(file, jobName string, entries []mapEntry, ref string) {
	uses, _ := entryValue(entries, "uses")
	switch actionName(ref) {
	case "actions/checkout":
		if ref != reviewedCheckout {
			v.addNodeError(file, uses, "Node policy checkout must use reviewed action ref %s", reviewedCheckout)
		}
	case "actions/setup-go":
		if ref != reviewedSetupGo {
			v.addNodeError(file, uses, "Node policy setup-go must use reviewed action ref %s", reviewedSetupGo)
		}
	}
	for _, entry := range entries {
		switch entry.key.Value {
		case "name", "uses", "with":
		default:
			v.addNodeError(file, entry.key, "bootstrap action %q step key %q is not allowed", ref, entry.key.Value)
		}
	}
	with, found := entryValue(entries, "with")
	if !found {
		v.addNodeError(file, nil, "bootstrap action %q is missing its reviewed inputs", ref)
		return
	}
	withEntries, ok := v.executableEntries(file, with, "Node bootstrap action with")
	if !ok {
		return
	}
	switch actionName(ref) {
	case "actions/checkout":
		v.checkNodeCheckoutInputs(file, jobName, withEntries, ref)
	case "actions/setup-go":
		v.checkNodeSetupGoInputs(file, withEntries, ref)
	}
}

func (v *verifier) checkNodeCheckoutInputs(file, jobName string, entries []mapEntry, ref string) {
	wantFetchDepth := jobName == "release-gate" || jobName == "release"
	for _, entry := range entries {
		name := entry.key.Value
		if name == "persist-credentials" || (wantFetchDepth && name == "fetch-depth") {
			continue
		}
		v.addNodeError(file, entry.key, "checkout input %q is not in the canonical allowlist for job %q", name, jobName)
	}
	persist, found := entryValue(entries, "persist-credentials")
	if !found || persist.Kind != yaml.ScalarNode || persist.Tag != "!!bool" || persist.Value != "false" {
		v.addNodeError(file, persist, "bootstrap checkout %q must set literal boolean persist-credentials: false", ref)
	}
	if !wantFetchDepth {
		return
	}
	depth, found := entryValue(entries, "fetch-depth")
	if !found || depth.Kind != yaml.ScalarNode || depth.Tag != "!!int" || depth.Value != "0" {
		v.addNodeError(file, depth, "checkout %q in job %q must set literal integer fetch-depth: 0", ref, jobName)
	}
}

func (v *verifier) checkNodeSetupGoInputs(file string, entries []mapEntry, ref string) {
	for _, entry := range entries {
		switch entry.key.Value {
		case "go-version", "cache":
		default:
			v.addNodeError(file, entry.key, "setup-go input %q is not in the canonical Node-bootstrap allowlist", entry.key.Value)
		}
	}
	version, found := entryValue(entries, "go-version")
	if !found || version.Kind != yaml.ScalarNode || version.Tag != "!!str" || version.Value != v.goMinimum {
		v.addNodeError(file, version, "bootstrap setup-go %q must set literal go-version: %s", ref, v.goMinimum)
	}
	cache, found := entryValue(entries, "cache")
	if !found || cache.Kind != yaml.ScalarNode || cache.Tag != "!!bool" || cache.Value != "true" {
		v.addNodeError(file, cache, "bootstrap setup-go %q must set literal boolean cache: true", ref)
	}
}

func (v *verifier) checkNodeBootstrapRun(file string, entries []mapEntry, command string) {
	for _, entry := range entries {
		switch entry.key.Value {
		case "name", "run":
		default:
			v.addNodeError(file, entry.key, "bootstrap run %q step key %q is not allowed", command, entry.key.Value)
		}
	}
}

func (v *verifier) checkNodeBootstrapComplete(file, jobName string, state nodeJobState) {
	if !state.checkoutSeen {
		v.addNodeError(file, nil, "Node policy job %q must contain one canonical checkout before setup-go", jobName)
	}
	if !state.setupGoSeen {
		v.addNodeError(file, nil, "Node policy job %q must contain one canonical setup-go after checkout", jobName)
	}
	if jobName == "release-gate" && !state.semgrepInstallSeen {
		v.addNodeError(file, nil, "release-gate must install pinned Semgrep exactly once before setup-node")
	}
}

func (v *verifier) checkNodeRuntimeCondition(file string, entries []mapEntry, command string) bool {
	valid := true
	condition, conditional := entryValue(entries, "if")
	if v.requiredRunIf == "" {
		if conditional {
			v.addNodeError(file, condition, "active Node verification %q must be unconditional", command)
			valid = false
		}
	} else if !conditional || condition.Kind != yaml.ScalarNode || condition.Tag != "!!str" || condition.Value != v.requiredRunIf {
		v.addNodeError(file, condition, "active Node verification %q must use the literal condition %q", command, v.requiredRunIf)
		valid = false
	}
	if continuation, found := entryValue(entries, "continue-on-error"); found {
		v.addNodeError(file, continuation, "active Node verification %q must not set continue-on-error", command)
		valid = false
	}
	return valid
}

func (v *verifier) checkRunExecutionContext(file string, entries []mapEntry, command string) {
	if shell, found := entryValue(entries, "shell"); found {
		v.addNodeError(file, shell, "Node policy run %q must use the runner's default shell", command)
	}
	if directory, found := entryValue(entries, "working-directory"); found {
		v.addNodeError(file, directory, "Node policy run %q must use the repository root working directory", command)
	}
	if environment, found := entryValue(entries, "env"); found {
		v.addNodeError(file, environment, "Node policy run %q must not define environment overrides", command)
	}
}

func (v *verifier) checkNodePolicyJob(file, jobName string, entries []mapEntry) {
	for _, entry := range entries {
		switch entry.key.Value {
		case "name", "runs-on", "steps":
		default:
			v.addNodeError(file, entry.key, "job %q key %q is not allowed on a Node policy job", jobName, entry.key.Value)
		}
	}
	runner, found := entryValue(entries, "runs-on")
	if !found || runner.Kind != yaml.ScalarNode || runner.Tag != "!!str" || runner.Value != "ubuntu-latest" {
		v.addNodeError(file, runner, "job %q must use the literal runner ubuntu-latest", jobName)
	}
}

func (v *verifier) checkSetupNodeStepKeys(file string, entries []mapEntry) bool {
	valid := true
	for _, entry := range entries {
		switch entry.key.Value {
		case "id", "name", "uses", "with":
		default:
			v.addNodeError(file, entry.key, "setup-node step key %q is not allowed", entry.key.Value)
			valid = false
		}
	}
	return valid
}

func (v *verifier) literalRunCommand(file string, run *yaml.Node) (string, bool) {
	if run.Kind != yaml.ScalarNode || run.Tag != "!!str" {
		v.addNodeError(file, run, "workflow run command must be a literal string")
		return "", false
	}
	command := strings.TrimSpace(run.Value)
	if command == "" {
		v.addNodeError(file, run, "workflow run command must not be empty")
		return "", false
	}
	return command, true
}

func (v *verifier) scanSteps(file string, steps *yaml.Node) {
	if steps.Kind != yaml.SequenceNode {
		v.addNodeError(file, steps, "unsupported or dynamic steps structure; steps must be a YAML sequence")
		return
	}
	inheritedComment := ""
	if len(steps.Content) == 1 {
		inheritedComment = steps.LineComment
	}
	for _, step := range steps.Content {
		if step.Kind != yaml.MappingNode {
			v.addNodeError(file, step, "unsupported workflow step structure; each step must be a YAML mapping")
			continue
		}
		v.scanStep(file, step, inheritedComment)
	}
}

func (v *verifier) scanStep(file string, step *yaml.Node, inheritedComment string) {
	entries, ok := v.executableEntries(file, step, "workflow step")
	if !ok {
		return
	}
	uses, found := entryValue(entries, "uses")
	if !found {
		return
	}
	v.inspectStepUses(file, step, entries, uses, inheritedComment)
}

func (v *verifier) scanLocalAction(file string, root *yaml.Node) {
	entries, ok := v.executableEntries(file, root, "local action metadata")
	if !ok {
		return
	}
	runs, found := entryValue(entries, "runs")
	if !found || runs.Kind != yaml.MappingNode {
		v.addNodeError(file, runs, "local action metadata must define a runs mapping")
		return
	}
	runEntries, ok := v.executableEntries(file, runs, "local action runs")
	if !ok {
		return
	}
	using, found := entryValue(runEntries, "using")
	if !found || using.Kind != yaml.ScalarNode || using.Tag != "!!str" || strings.TrimSpace(using.Value) == "" {
		v.addNodeError(file, using, "local action metadata must define a literal runs.using value")
		return
	}
	if using.Value != "composite" {
		return
	}
	steps, found := entryValue(runEntries, "steps")
	if !found {
		v.addNodeError(file, runs, "composite action metadata is missing runs.steps")
		return
	}
	v.scanSteps(file, steps)
}

func (v *verifier) inspectStepUses(file string, step *yaml.Node, stepEntries []mapEntry, value *yaml.Node, inheritedComment string) {
	ref, ok := v.literalReference(file, value, "step uses")
	if !ok {
		return
	}

	if strings.HasPrefix(ref, "./") {
		actionFile, err := v.resolveLocalAction(ref)
		if err != nil {
			v.addNodeError(file, value, "local action %s: %v", ref, err)
			return
		}
		v.scanFile(actionFile, value, actionKind)
		return
	}

	if v.mode == modeActions {
		v.checkExternalAction(file, step, value, ref, inheritedComment)
	}

	if v.mode == modeGo && strings.EqualFold(actionName(ref), "actions/setup-go") {
		v.setupGoCount++
		v.checkSetupGo(file, stepEntries, value)
	}
}

func (v *verifier) inspectJobUses(file string, job, value *yaml.Node) {
	ref, ok := v.literalReference(file, value, "job uses")
	if !ok {
		return
	}
	if strings.HasPrefix(ref, "./") {
		if !isWorkflowReference(ref) {
			v.addNodeError(file, value, "local reusable workflow reference must name a .yml or .yaml file: %s", ref)
			return
		}
		workflowFile, err := v.resolveLocalWorkflow(ref)
		if err != nil {
			v.addNodeError(file, value, "local reusable workflow %s: %v", ref, err)
			return
		}
		v.scanFile(workflowFile, value, workflowKind)
		return
	}
	if v.mode == modeActions {
		v.checkExternalAction(file, job, value, ref, "")
	}
}

func (v *verifier) literalReference(file string, value *yaml.Node, context string) (string, bool) {
	if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
		v.addNodeError(file, value, "unsupported or dynamic %s; value must be a literal string", context)
		return "", false
	}
	ref := value.Value
	if strings.TrimSpace(ref) != ref || strings.ContainsAny(ref, "\r\n") || strings.ContainsAny(ref, "${}") {
		v.addNodeError(file, value, "unsupported or dynamic %s: %s", context, ref)
		return "", false
	}
	return ref, true
}

func actionName(ref string) string {
	if at := strings.IndexByte(ref, '@'); at >= 0 {
		return ref[:at]
	}
	return ref
}

func isWorkflowReference(ref string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSuffix(ref, "/")))
	return ext == ".yml" || ext == ".yaml"
}

func (v *verifier) checkExternalAction(file string, step, node *yaml.Node, ref, inheritedComment string) {
	if !fullSHARef.MatchString(ref) {
		v.addNodeError(file, node, "external action is not pinned to a full commit SHA: %s", ref)
		return
	}
	if _, retired := retiredRuntimeActions[strings.ToLower(ref)]; retired {
		v.addNodeError(file, node, "action uses a retired Node runtime: %s", ref)
	}
	comments := []string{node.LineComment}
	if step != nil {
		comments = append(comments, step.LineComment)
	}
	comments = append(comments, inheritedComment)
	if !hasRenovateVersionComment(comments...) {
		v.addNodeError(file, node, "SHA-pinned action is missing a Renovate version comment: %s", ref)
	}
}

func hasRenovateVersionComment(comments ...string) bool {
	for _, comment := range comments {
		comment = strings.TrimSpace(comment)
		comment = strings.TrimSpace(strings.TrimPrefix(comment, "#"))
		if commentVer.MatchString(comment) {
			return true
		}
	}
	return false
}

func (v *verifier) checkSetupGo(file string, stepEntries []mapEntry, uses *yaml.Node) {
	with, found := entryValue(stepEntries, "with")
	if !found || (with.Kind == yaml.ScalarNode && with.Tag == "!!null") {
		v.addNodeError(file, uses, "setup-go step is missing with.go-version: %s", v.goMinimum)
		return
	}
	withEntries, ok := v.executableEntries(file, with, "setup-go with")
	if !ok {
		return
	}
	version, found := entryValue(withEntries, "go-version")
	if !found {
		v.addNodeError(file, uses, "setup-go step is missing with.go-version: %s", v.goMinimum)
		return
	}
	if version.Kind != yaml.ScalarNode || version.Tag != "!!str" || strings.ContainsAny(version.Value, "${}") {
		v.addNodeError(file, version, "setup-go go-version must be the literal %s", v.goMinimum)
		return
	}
	if version.Value != v.goMinimum {
		v.addNodeError(file, version, "go-version %s does not match root security minimum %s", version.Value, v.goMinimum)
	}
}

func (v *verifier) checkSetupNode(file string, stepEntries []mapEntry, uses *yaml.Node) bool {
	with, found := entryValue(stepEntries, "with")
	if !found || (with.Kind == yaml.ScalarNode && with.Tag == "!!null") {
		v.addNodeError(file, uses, "setup-node step is missing with.node-version-file: %s", v.nodeFile)
		return false
	}
	withEntries, ok := v.executableEntries(file, with, "setup-node with")
	if !ok {
		return false
	}
	valid := v.checkSetupNodeInputNames(file, withEntries)
	versionFile, found := entryValue(withEntries, "node-version-file")
	if !found {
		v.addNodeError(file, uses, "setup-node step is missing with.node-version-file: %s", v.nodeFile)
		valid = false
	} else if versionFile.Kind != yaml.ScalarNode || versionFile.Tag != "!!str" || strings.ContainsAny(versionFile.Value, "${}") {
		v.addNodeError(file, versionFile, "setup-node node-version-file must be the literal %s", v.nodeFile)
		valid = false
	} else if versionFile.Value != v.nodeFile {
		v.addNodeError(file, versionFile, "node-version-file %s does not match shared Node version file %s", versionFile.Value, v.nodeFile)
		valid = false
	}

	cache, found := entryValue(withEntries, "package-manager-cache")
	if !found {
		v.addNodeError(file, uses, "setup-node step is missing with.package-manager-cache: false")
		valid = false
	} else if cache.Kind != yaml.ScalarNode || cache.Tag != "!!bool" || cache.Value != "false" {
		v.addNodeError(file, cache, "setup-node package-manager-cache must be the literal boolean false")
		valid = false
	}
	return valid
}

func (v *verifier) checkSetupNodeInputNames(file string, entries []mapEntry) bool {
	valid := true
	for _, entry := range entries {
		name := entry.key.Value
		switch {
		case name == "node-version-file", name == "package-manager-cache":
			continue
		case strings.EqualFold(name, "node-version"):
			v.addNodeError(file, entry.key, "setup-node input %q must not override the shared node-version-file", name)
			valid = false
		case strings.EqualFold(name, "node-version-file") && name != "node-version-file":
			v.addNodeError(file, entry.key, "setup-node input %q must use the exact lowercase key node-version-file", name)
			valid = false
		case strings.EqualFold(name, "package-manager-cache") && name != "package-manager-cache":
			v.addNodeError(file, entry.key, "setup-node input %q must use the exact lowercase key package-manager-cache", name)
			valid = false
		default:
			v.addNodeError(file, entry.key, "setup-node input %q is not in the reviewed allowlist", name)
			valid = false
		}
	}
	return valid
}

func (v *verifier) executableEntries(file string, node *yaml.Node, context string) ([]mapEntry, bool) {
	if node == nil || node.Kind != yaml.MappingNode {
		v.addNodeError(file, node, "%s must be a YAML mapping", context)
		return nil, false
	}
	if len(node.Content)%2 != 0 {
		v.addNodeError(file, node, "malformed %s mapping structure", context)
		return nil, false
	}
	entries := make([]mapEntry, 0, len(node.Content)/2)
	seen := make(map[string]*yaml.Node, len(node.Content)/2)
	valid := true
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			v.addNodeError(file, key, "unsupported non-string key in %s", context)
			valid = false
			continue
		}
		if first, duplicate := seen[key.Value]; duplicate {
			v.addNodeError(file, key, "duplicate key %q in %s; first declared at line %d", key.Value, context, first.Line)
			valid = false
			continue
		}
		seen[key.Value] = key
		entries = append(entries, mapEntry{key: key, value: node.Content[i+1]})
	}
	return entries, valid
}

func entryValue(entries []mapEntry, name string) (*yaml.Node, bool) {
	for _, entry := range entries {
		if entry.key.Value == name {
			return entry.value, true
		}
	}
	return nil, false
}

func (v *verifier) resolveLocalAction(ref string) (string, error) {
	if filepath.IsAbs(ref) || strings.Contains(ref, "\\") {
		return "", fmt.Errorf("path is not a repository-relative POSIX path")
	}
	relative := strings.TrimPrefix(ref, "./")
	if relative == "" {
		return "", fmt.Errorf("path is empty")
	}
	candidate := filepath.Clean(filepath.Join(v.rootReal, filepath.FromSlash(relative)))
	if err := ensureInside(v.rootReal, candidate); err != nil {
		return "", fmt.Errorf("path escapes repository: %v", err)
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return "", fmt.Errorf("metadata directory not found: %v", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a local action directory")
	}
	realDir, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve metadata directory: %v", err)
	}
	if err := ensureInside(v.rootReal, realDir); err != nil {
		return "", fmt.Errorf("path escapes repository through a symlink: %v", err)
	}

	var metadata string
	for _, name := range []string{"action.yml", "action.yaml"} {
		path := filepath.Join(realDir, name)
		if _, err := os.Stat(path); err == nil {
			if metadata != "" {
				return "", fmt.Errorf("both action.yml and action.yaml exist")
			}
			metadata = path
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect %s: %v", name, err)
		}
	}
	if metadata == "" {
		return "", fmt.Errorf("action.yml/action.yaml metadata not found")
	}
	return metadata, nil
}

func (v *verifier) resolveLocalWorkflow(ref string) (string, error) {
	if filepath.IsAbs(ref) || strings.Contains(ref, "\\") {
		return "", fmt.Errorf("path is not a repository-relative POSIX path")
	}
	relative := strings.TrimPrefix(ref, "./")
	if relative == "" {
		return "", fmt.Errorf("path is empty")
	}
	candidate := filepath.Clean(filepath.Join(v.rootReal, filepath.FromSlash(relative)))
	if err := ensureInside(v.rootReal, candidate); err != nil {
		return "", fmt.Errorf("path escapes repository: %v", err)
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return "", fmt.Errorf("workflow file not found: %v", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a workflow file")
	}
	realFile, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve workflow file: %v", err)
	}
	if err := ensureInside(v.rootReal, realFile); err != nil {
		return "", fmt.Errorf("path escapes repository through a symlink: %v", err)
	}
	return realFile, nil
}

func ensureInside(root, target string) error {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return fmt.Errorf("compare repository boundary: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("target %q is outside repository root %q", target, root)
	}
	return nil
}

func (v *verifier) cycleDescription(target string) string {
	start := 0
	for i, path := range v.stack {
		if path == target {
			start = i
			break
		}
	}
	paths := append([]string{}, v.stack[start:]...)
	paths = append(paths, target)
	return strings.Join(paths, " -> ")
}

func (v *verifier) addNodeError(file string, node *yaml.Node, format string, args ...any) {
	line, column := nodePosition(node)
	v.addError(file, line, column, format, args...)
}

func (v *verifier) addErrorWithYAMLLine(file string, err error, format string) {
	line := 1
	if matches := lineInErr.FindStringSubmatch(err.Error()); len(matches) == 2 {
		if parsed, parseErr := strconv.Atoi(matches[1]); parseErr == nil {
			line = parsed
		}
	}
	v.addError(file, line, 1, format, err)
}

func (v *verifier) addError(file string, line, column int, format string, args ...any) {
	if line < 1 {
		line = 1
	}
	if column < 1 {
		column = 1
	}
	v.errors = append(v.errors, fmt.Sprintf("%s:%d:%d: %s", file, line, column, fmt.Sprintf(format, args...)))
}

func nodePosition(node *yaml.Node) (int, int) {
	if node == nil {
		return 1, 1
	}
	return node.Line, node.Column
}
