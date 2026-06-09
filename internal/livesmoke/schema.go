package livesmoke

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FieldSpec, Operation, and ResourceSpec mirror the JSON shape emitted by
// `zscalerctl --format json schema list`.
type FieldSpec struct {
	Name         string   `json:"name"`
	JSONName     string   `json:"json_name"`
	AllowedModes []string `json:"allowed_modes"`
}

type Operation struct {
	Name       string `json:"name"`
	Capability string `json:"capability"`
}

type ResourceSpec struct {
	Product    string      `json:"product"`
	Name       string      `json:"name"`
	Operations []Operation `json:"operations"`
	Fields     []FieldSpec `json:"fields"`
}

var smokeProducts = map[string]bool{
	"zia": true, "zpa": true, "ztw": true, "zcc": true, "zidentity": true,
}

func parseSchema(body []byte) ([]ResourceSpec, error) {
	var specs []ResourceSpec
	if err := json.Unmarshal(body, &specs); err != nil {
		return nil, fmt.Errorf("schema list is not valid JSON: %w", err)
	}
	return specs, nil
}

func findSpec(specs []ResourceSpec, product, name string) *ResourceSpec {
	for i := range specs {
		if specs[i].Product == product && specs[i].Name == name {
			return &specs[i]
		}
	}
	return nil
}

// hasReadOp reports whether the spec exposes a read operation with the given
// name (e.g. "list" or "show").
func (s *ResourceSpec) hasReadOp(name string) bool {
	for _, op := range s.Operations {
		if op.Name == name && op.Capability == "read" {
			return true
		}
	}
	return false
}

// readOperation returns the operation used to read the resource: "list" if it
// supports list, else "show" if it supports show, else "".
func (s *ResourceSpec) readOperation() string {
	switch {
	case s.hasReadOp("list"):
		return "list"
	case s.hasReadOp("show"):
		return "show"
	default:
		return ""
	}
}

// catalogReadResources returns every product/name in the catalog that supports a
// list or show read operation, sorted by product then name.
func catalogReadResources(specs []ResourceSpec) []string {
	var out []string
	for i := range specs {
		s := &specs[i]
		if !smokeProducts[s.Product] {
			continue
		}
		if s.hasReadOp("list") || s.hasReadOp("show") {
			out = append(out, s.Product+"/"+s.Name)
		}
	}
	sort.Strings(out)
	return out
}

func resourceProduct(qualified string) string {
	if i := strings.IndexByte(qualified, '/'); i >= 0 {
		return qualified[:i]
	}
	return qualified
}

func resourceName(qualified string) string {
	if i := strings.IndexByte(qualified, '/'); i >= 0 {
		return qualified[i+1:]
	}
	return qualified
}

// normalizeRequestedResource applies the same rules as the shell: bare names
// become zia/<name>; qualified names must use a known product prefix.
func normalizeRequestedResource(entry string) (string, error) {
	resource := strings.TrimSpace(entry)
	if resource == "" {
		return "", fmt.Errorf("--resources contains an empty entry")
	}
	if i := strings.IndexByte(resource, '/'); i >= 0 {
		product := resource[:i]
		if !smokeProducts[product] {
			return "", fmt.Errorf("--resources supports only zia/, zpa/, ztw/, zcc/, or zidentity/ qualified resources; got: %s", resource)
		}
		if resource[i+1:] == "" {
			return "", fmt.Errorf("--resources contains an empty resource name")
		}
		return resource, nil
	}
	return "zia/" + resource, nil
}

// parseResourceList splits a comma-separated --resources value into normalized
// qualified resources.
func parseResourceList(list string) ([]string, error) {
	var out []string
	for _, entry := range strings.Split(list, ",") {
		normalized, err := normalizeRequestedResource(entry)
		if err != nil {
			return nil, err
		}
		out = append(out, normalized)
	}
	return out, nil
}

// parseManifest reads a line-oriented manifest: comments after '#', Markdown
// bullets ("- " / "* "), and comma-separated entries are accepted.
func parseManifest(body []byte) ([]string, error) {
	var out []string
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			line = strings.TrimSpace(line[1:])
		}
		if line == "" {
			continue
		}
		entries, err := parseResourceList(line)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("live smoke manifest contains no resources")
	}
	return out, nil
}
