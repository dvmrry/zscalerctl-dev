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
	modeActions           = "actions"
	modeGo                = "setup-go"
	modeNode              = "setup-node"
	releaseCheckCondition = "steps.version.outputs.release == 'true'"
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
	mode            string
	goMinimum       string
	nodeFile        string
	requiredRun     string
	requiredRunJob  string
	requiredRunIf   string
	requiredRunFile string
	rootReal        string

	active  map[string]bool
	visited map[visitKey]bool
	stack   []string
	errors  []string

	setupGoCount      int
	setupNodeCount    int
	nodeConsumerCount int
	requiredRunCount  int
}

type mapEntry struct {
	key   *yaml.Node
	value *yaml.Node
}

type visitKey struct {
	path string
	kind fileKind
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
	if *mode != modeNode && (*requiredRun != "" || *requiredRunJob != "" || *requiredRunIf != "") {
		failUsage("--required-run, --required-run-job, and --required-run-if are valid only in setup-node mode")
	}
	if (*requiredRun == "") != (*requiredRunJob == "") {
		failUsage("--required-run and --required-run-job must be provided together")
	}
	if *requiredRunIf != "" && *requiredRun == "" {
		failUsage("--required-run-if requires --required-run and --required-run-job")
	}

	v, err := newVerifier(*mode, *repoRoot, *goMinimum, *nodeFile, *requiredRun, *requiredRunJob, *requiredRunIf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify-workflow-policies: %v\n", err)
		os.Exit(1)
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
	fmt.Fprintln(os.Stderr, "usage: go run ./scripts/verify-workflow-policies.go --mode actions|setup-go|setup-node --scan-dir DIR --repo-root DIR [--go-minimum VERSION] [--node-version-file PATH] [--required-run COMMAND --required-run-job JOB [--required-run-if CONDITION]]")
	os.Exit(2)
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
	jobs, found := entryValue(entries, "jobs")
	if !found {
		v.addNodeError(file, root, "workflow is missing jobs mapping")
		return
	}
	jobEntries, ok := v.executableEntries(file, jobs, "workflow jobs")
	if !ok {
		return
	}
	for _, job := range jobEntries {
		if job.value.Kind != yaml.MappingNode {
			v.addNodeError(file, job.value, "workflow job %q must be a YAML mapping", job.key.Value)
			continue
		}
		v.scanJob(file, job.key.Value, job.value)
	}
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
				if condition, conditional := entryValue(entries, "if"); conditional {
					v.addNodeError(file, condition, "job %q containing Node policy steps must be unconditional", jobName)
				}
				if continuation, found := entryValue(entries, "continue-on-error"); found {
					v.addNodeError(file, continuation, "job %q containing Node policy steps must not set continue-on-error", jobName)
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

	setupReady := false
	relevant := false
	for _, step := range steps.Content {
		if step.Kind != yaml.MappingNode {
			v.addNodeError(file, step, "unsupported workflow step structure; each step must be a YAML mapping")
			continue
		}
		var stepRelevant bool
		setupReady, stepRelevant = v.scanNodeStep(file, jobName, step, setupReady)
		relevant = relevant || stepRelevant
	}
	return relevant
}

func (v *verifier) scanNodeStep(file, jobName string, step *yaml.Node, setupReady bool) (bool, bool) {
	entries, ok := v.executableEntries(file, step, "workflow step")
	if !ok {
		return setupReady, false
	}
	uses, hasUses := entryValue(entries, "uses")
	run, hasRun := entryValue(entries, "run")
	if hasUses && hasRun {
		v.addNodeError(file, step, "workflow step cannot define both uses and run")
		return setupReady, false
	}

	if hasUses {
		ref, ok := v.literalReference(file, uses, "step uses")
		if !ok || !strings.EqualFold(actionName(ref), "actions/setup-node") {
			return setupReady, false
		}

		v.setupNodeCount++
		valid := true
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
		return setupReady || valid, true
	}

	if !hasRun {
		return setupReady, false
	}
	command, ok := v.literalRunCommand(file, run)
	if !ok {
		return setupReady, false
	}

	relevant := false
	if isNodeConsumerCommand(command) {
		relevant = true
		v.nodeConsumerCount++
		if !setupReady {
			v.addNodeError(
				file,
				run,
				"Node consumer %q must follow a valid unconditional direct setup-node step in the same job",
				command,
			)
		}
		v.checkNodeConsumerStep(file, entries, run, command)
	}

	if filepath.Clean(file) != v.requiredRunFile || command != v.requiredRun {
		return setupReady, relevant
	}
	relevant = true
	if jobName != v.requiredRunJob {
		v.addNodeError(file, run, "required run %q must be in workflow job %q", command, v.requiredRunJob)
		return setupReady, relevant
	}
	condition, conditional := entryValue(entries, "if")
	if v.requiredRunIf == "" {
		if conditional {
			v.addNodeError(file, condition, "required run %q must be unconditional", command)
			return setupReady, relevant
		}
	} else if !conditional || condition.Kind != yaml.ScalarNode || condition.Tag != "!!str" || condition.Value != v.requiredRunIf {
		v.addNodeError(file, condition, "required run %q must use the literal condition %q", command, v.requiredRunIf)
		return setupReady, relevant
	}
	if continuation, found := entryValue(entries, "continue-on-error"); found {
		v.addNodeError(file, continuation, "required run %q must not set continue-on-error", command)
		return setupReady, relevant
	}
	v.requiredRunCount++
	return setupReady, relevant
}

func isNodeConsumerCommand(command string) bool {
	switch command {
	case "bash scripts/verify-typescript-client.sh",
		"make check",
		"make release-check",
		"make verify-typescript-client":
		return true
	default:
		return false
	}
}

func (v *verifier) checkNodeConsumerStep(file string, entries []mapEntry, run *yaml.Node, command string) {
	if continuation, found := entryValue(entries, "continue-on-error"); found {
		v.addNodeError(file, continuation, "Node consumer %q must not set continue-on-error", command)
	}

	condition, conditional := entryValue(entries, "if")
	if command != "make release-check" {
		if conditional {
			v.addNodeError(file, condition, "Node consumer %q must be unconditional", command)
		}
		return
	}
	if !conditional {
		v.addNodeError(file, run, "Node consumer %q must use the literal release condition %q", command, releaseCheckCondition)
		return
	}
	if condition.Kind != yaml.ScalarNode || condition.Tag != "!!str" || condition.Value != releaseCheckCondition {
		v.addNodeError(file, condition, "Node consumer %q must use the literal release condition %q", command, releaseCheckCondition)
	}
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
		case strings.EqualFold(name, "node-version"):
			v.addNodeError(file, entry.key, "setup-node input %q must not override the shared node-version-file", name)
			valid = false
		case strings.EqualFold(name, "node-version-file") && name != "node-version-file":
			v.addNodeError(file, entry.key, "setup-node input %q must use the exact lowercase key node-version-file", name)
			valid = false
		case strings.EqualFold(name, "package-manager-cache") && name != "package-manager-cache":
			v.addNodeError(file, entry.key, "setup-node input %q must use the exact lowercase key package-manager-cache", name)
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
