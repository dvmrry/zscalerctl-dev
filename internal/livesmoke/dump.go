package livesmoke

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// readArtifact reads a dump artifact (resource file, manifest, report) from the
// validator's own output directory.
func readArtifact(path string) ([]byte, error) {
	// #nosec G304 -- path is a dump artifact under the tool's own output dir,
	// not external request input.
	return os.ReadFile(path)
}

type dumpManifestResource struct {
	Product string `json:"product"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Records int    `json:"records"`
}

type dumpManifest struct {
	Warning    string                 `json:"warning"`
	Status     string                 `json:"status"`
	Errors     int                    `json:"errors"`
	ErrorsPath string                 `json:"errors_path"`
	Resources  []dumpManifestResource `json:"resources"`
}

type redactionReport struct {
	Resources []struct {
		Product        string   `json:"product"`
		Name           string   `json:"name"`
		DroppedFields  []string `json:"dropped_fields"`
		RedactedFields []string `json:"redacted_fields"`
	} `json:"resources"`
}

// validateDump runs the dump command and validates the resulting directory:
// permissions, manifest, redaction report, per-resource files, and counts.
func (s *smoke) validateDump() {
	dumpDir := filepath.Join(s.outDir, "dump")
	args := []string{"dump", "--products", strings.Join(s.selectedProducts(), ",")}
	if len(s.requested) > 0 {
		args = append(args, "--resources", strings.Join(s.resources, ","))
	}
	args = append(args, "--out", dumpDir)

	start := s.rep.failures
	_, stderr, code := s.runner.Run(args...)
	if code != 0 {
		s.captureStderr("dump.stderr", stderr)
		s.rep.fail("dump command failed")
	} else {
		s.rep.pass("dump command completed")
	}
	s.rep.recordFromFailures("dump", "command", start, "-", "dump", "see dump.stderr")

	if _, err := os.Stat(dumpDir); err != nil {
		return
	}

	manifestStart := s.rep.failures
	s.validateFileMode("dump root directory", dumpDir, "700")
	s.validateFileMode("dump resources directory", filepath.Join(dumpDir, "resources"), "700")
	for _, product := range s.selectedProducts() {
		s.validateFileMode("dump "+product+" directory", filepath.Join(dumpDir, "resources", product), "700")
	}

	manifestPath := filepath.Join(dumpDir, "manifest.json")
	manifest := s.validateManifest(manifestPath)
	s.validateRedactionReport(filepath.Join(dumpDir, "redaction_report.json"))

	if _, err := os.Stat(filepath.Join(dumpDir, "errors.ndjson")); err == nil {
		s.rep.fail("complete dump unexpectedly wrote errors.ndjson")
	} else {
		s.rep.pass("complete dump did not write errors.ndjson")
	}
	s.rep.recordFromFailures("dump", "manifest", manifestStart, fmt.Sprintf("%d", len(s.resources)), "complete", "see dump artifacts")

	filesStart := s.rep.failures
	s.validateDumpFileSet(dumpDir)
	s.rep.recordFromFailures("dump", "files", filesStart, fmt.Sprintf("%d", len(s.resources)), "resource set matches", "see dump artifacts")

	for _, qualified := range s.resources {
		s.validateDumpResource(dumpDir, qualified)
	}

	if manifest != nil {
		s.crossCheckManifestCounts(dumpDir, manifest)
	}
}

func (s *smoke) validateManifest(path string) *dumpManifest {
	body, err := readArtifact(path)
	if err != nil {
		s.rep.fail("dump manifest missing: %s", path)
		return nil
	}
	s.validateFileMode("dump manifest", path, "600")

	var manifest dumpManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		s.rep.fail("dump manifest is not valid JSON: %s", path)
		return nil
	}
	s.rep.pass("dump manifest is valid JSON")

	if manifest.Warning == manifestWarning {
		s.rep.pass("dump manifest includes confidentiality warning")
	} else {
		s.rep.fail("dump manifest missing confidentiality warning")
	}
	if manifest.Status == "complete" {
		s.rep.pass("dump manifest status is complete")
	} else {
		s.rep.fail("dump manifest status is not complete")
	}
	if manifest.Errors == 0 && manifest.ErrorsPath == "" {
		s.rep.pass("dump manifest has no partial-error metadata")
	} else {
		s.rep.fail("dump manifest includes unexpected partial-error metadata")
	}

	expected := s.expectedDumpPaths()
	var actual []string
	for _, r := range manifest.Resources {
		actual = append(actual, r.Path)
	}
	if reflect.DeepEqual(sortedCopy(expected), sortedCopy(actual)) {
		s.rep.pass("dump manifest resource set matches selected resources")
	} else {
		s.rep.fail("dump manifest resource set differs from selected resources")
	}
	return &manifest
}

func (s *smoke) validateRedactionReport(path string) {
	body, err := readArtifact(path)
	if err != nil {
		s.rep.fail("redaction report missing: %s", path)
		return
	}
	s.validateFileMode("redaction report", path, "600")

	var report redactionReport
	if err := json.Unmarshal(body, &report); err != nil {
		s.rep.fail("redaction report is not valid JSON: %s", path)
		return
	}
	s.rep.pass("redaction report is valid JSON")
	if strings.Contains(string(body), "<REDACTED:") {
		s.rep.fail("redaction report contains redaction marker values")
	} else {
		s.rep.pass("redaction report is value-free")
	}

	found := false
	for _, r := range report.Resources {
		if len(r.DroppedFields) > 0 || len(r.RedactedFields) > 0 {
			found = true
			s.rep.info("redaction report %s %s: dropped fields [%s], redacted fields [%s]",
				r.Product, r.Name, strings.Join(r.DroppedFields, ","), strings.Join(r.RedactedFields, ","))
		}
	}
	if !found {
		s.rep.pass("redaction report has no dropped or redacted field entries")
	}
}

func (s *smoke) expectedDumpPaths() []string {
	var out []string
	for _, q := range s.resources {
		out = append(out, fmt.Sprintf("resources/%s/%s.json", resourceProduct(q), resourceName(q)))
	}
	return out
}

func (s *smoke) validateDumpFileSet(dumpDir string) {
	resourcesDir := filepath.Join(dumpDir, "resources")
	if _, err := os.Stat(resourcesDir); err != nil {
		s.rep.fail("dump resources directory missing: %s", resourcesDir)
		return
	}
	var actual []string
	_ = filepath.Walk(resourcesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		rel, rerr := filepath.Rel(dumpDir, path)
		if rerr == nil {
			actual = append(actual, filepath.ToSlash(rel))
		}
		return nil
	})
	if reflect.DeepEqual(sortedCopy(s.expectedDumpPaths()), sortedCopy(actual)) {
		s.rep.pass("dump resource files match selected resources")
	} else {
		s.rep.fail("dump resource files differ from selected resources")
	}
}

func (s *smoke) validateDumpResource(dumpDir, qualified string) {
	product := resourceProduct(qualified)
	name := resourceName(qualified)
	artifact := strings.ReplaceAll(qualified, "/", "-")
	file := filepath.Join(dumpDir, "resources", product, name+".json")
	spec := findSpec(s.specs, product, name)
	operation := ""
	if spec != nil {
		operation = spec.readOperation()
	}

	start := s.rep.failures
	body, err := readArtifact(file)
	if err != nil {
		s.rep.fail("dump resource file missing: %s", file)
		s.rep.record(qualified, "dump", "FAIL", "-", "missing dump file")
		return
	}
	s.validateFileMode(fmt.Sprintf("dump %s %s file", product, name), file, "600")

	data, perr := decodeJSON(body)
	label := fmt.Sprintf("dump %s %s", product, name)
	records := "-"
	if operation == "show" {
		if perr == nil && s.validateObject(label, data) {
			records = "1"
			s.runFieldChecks(label, product, name, data)
			s.compareCounts(qualified, operation, artifact, 1)
		} else if perr != nil {
			s.rep.fail("%s is not a JSON object: %v", label, perr)
		}
	} else {
		if perr == nil && s.validateArray(label, data) {
			n := arrayLen(data)
			records = fmt.Sprintf("%d", n)
			s.runFieldChecks(label, product, name, data)
			s.compareCounts(qualified, operation, artifact, n)
		} else if perr != nil {
			s.rep.fail("%s is not a JSON array: %v", label, perr)
		}
	}
	s.rep.recordFromFailures(qualified, "dump", start, records, "", "see dump file")
}

func (s *smoke) compareCounts(qualified, operation, artifact string, dumpCount int) {
	listCount, ok := s.listCounts[artifact]
	if !ok {
		return
	}
	if listCount == dumpCount {
		s.rep.pass("%s %s and dump counts match (%d records)", qualified, operation, dumpCount)
		return
	}
	if s.opts.StrictCounts {
		s.rep.fail("%s %s count = %d, dump count = %d", qualified, operation, listCount, dumpCount)
	} else {
		s.rep.info("%s %s count = %d, dump count = %d; live data may have changed between reads", qualified, operation, listCount, dumpCount)
	}
}

func (s *smoke) crossCheckManifestCounts(dumpDir string, manifest *dumpManifest) {
	for _, r := range manifest.Resources {
		file := filepath.Join(dumpDir, filepath.FromSlash(r.Path))
		body, err := readArtifact(file)
		if err != nil {
			s.rep.fail("manifest references missing resource file: %s", r.Path)
			continue
		}
		data, perr := decodeJSON(body)
		got := -1
		if perr == nil {
			switch t := data.(type) {
			case []any:
				got = len(t)
			case map[string]any:
				got = 1
			}
		}
		if got == r.Records {
			s.rep.pass("manifest count matches %s (%d records)", r.Path, got)
		} else {
			s.rep.fail("manifest count for %s = %d, file has %d", r.Path, r.Records, got)
		}
	}
}
