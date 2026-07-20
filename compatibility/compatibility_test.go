package compatibility

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type matrixEntry struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Status string `json:"status"`
}

type openAPIDoc struct {
	Paths map[string]pathItem `yaml:"paths"`
}

type pathItem struct {
	Get    *operation `yaml:"get"`
	Post   *operation `yaml:"post"`
	Put    *operation `yaml:"put"`
	Patch  *operation `yaml:"patch"`
	Delete *operation `yaml:"delete"`
	Head   *operation `yaml:"head"`
}

type operation struct {
	Tags []string `yaml:"tags"`
}

var pathParameter = regexp.MustCompile(`\{[^}]+\}`)

func operationKey(method, path string) string {
	normalized := pathParameter.ReplaceAllString(path, "{}")
	normalized = strings.Replace(normalized, "/orderflow/{}", "/orderflow/orderflow", 1)
	return strings.ToUpper(method) + " " + normalized
}

func TestCompatibilityMatrixCoversCurrentContract(t *testing.T) {
	var matrix []matrixEntry
	readJSON(t, "matrix.json", &matrix)

	var upstream openAPIDoc
	readYAML(t, "upstream-openapi.yaml", &upstream)

	expected := operationKeys(upstream, func(method, path string, tags []string) bool {
		return contains(tags, "Research") || method == "get" && path == "/negotiate"
	})
	actual := make([]string, 0, len(matrix))
	ids := map[string]bool{}
	for _, entry := range matrix {
		if entry.ID == "" || ids[entry.ID] {
			t.Fatalf("matrix id is empty or duplicated: %q", entry.ID)
		}
		ids[entry.ID] = true
		actual = append(actual, operationKey(entry.Method, entry.Path))
	}
	sort.Strings(actual)

	if strings.Join(expected, "\n") != strings.Join(actual, "\n") {
		t.Fatalf("compatibility matrix is stale\nexpected:\n%s\n\nactual:\n%s", strings.Join(expected, "\n"), strings.Join(actual, "\n"))
	}
}

func TestCompatibilityClassificationsMatchFakerSpec(t *testing.T) {
	var matrix []matrixEntry
	readJSON(t, "matrix.json", &matrix)

	var faker openAPIDoc
	readYAML(t, "../api/openapi.yaml", &faker)
	implemented := map[string]bool{}
	for _, key := range operationKeys(faker, nil) {
		implemented[key] = true
	}

	for _, entry := range matrix {
		present := implemented[operationKey(entry.Method, entry.Path)]
		if entry.Status == "missing" && present {
			t.Errorf("%s is classified missing but now exists; update its compatibility review", entry.ID)
		}
		if entry.Status != "missing" && !present {
			t.Errorf("%s is classified %s but its route disappeared", entry.ID, entry.Status)
		}
	}
}

func TestLiveProbeFixturesAreSanitized(t *testing.T) {
	files, err := filepath.Glob("live-probes/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no live probe fixtures")
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var fixture struct {
			Sanitized bool `json:"sanitized"`
		}
		if err := json.Unmarshal(data, &fixture); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !fixture.Sanitized {
			t.Errorf("%s is not marked sanitized", path)
		}
		for _, forbidden := range []string{"Bearer ", "Basic ", "access_token", "gexbot_custom_"} {
			if strings.Contains(string(data), forbidden) {
				t.Errorf("%s contains forbidden secret marker %q", path, forbidden)
			}
		}
	}
}

func operationKeys(doc openAPIDoc, exclude func(string, string, []string) bool) []string {
	var keys []string
	for path, item := range doc.Paths {
		operations := map[string]*operation{
			"get": item.Get, "post": item.Post, "put": item.Put,
			"patch": item.Patch, "delete": item.Delete, "head": item.Head,
		}
		for method, operation := range operations {
			if operation == nil {
				continue
			}
			if exclude != nil && exclude(method, path, operation.Tags) {
				continue
			}
			keys = append(keys, operationKey(method, path))
		}
	}
	sort.Strings(keys)
	return keys
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}

func readYAML(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
