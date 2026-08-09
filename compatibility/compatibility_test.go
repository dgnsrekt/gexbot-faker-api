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
		// /negotiate (all methods) is the Azure Web PubSub handshake — a WebSocket
		// concern the faker serves outside the REST/OpenAPI surface, so it is not
		// tracked for REST parity (see README).
		return contains(tags, "Research") || path == "/negotiate"
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

// schemaDoc extracts component schema property names from an OpenAPI spec.
type schemaDoc struct {
	Components struct {
		Schemas map[string]struct {
			Properties map[string]yaml.Node `yaml:"properties"`
		} `yaml:"schemas"`
	} `yaml:"components"`
}

type liveProbeDoc struct {
	ObservedAt string `json:"observed_at"`
	Probes     []struct {
		Operation string   `json:"operation"`
		Status    int      `json:"status"`
		Fields    []string `json:"fields"`
	} `json:"probes"`
}

// schemaForOperation maps a live-probe operation to the faker response schema
// that should describe its body, or "" if the faker does not serve it. The live
// API prefixes market-data routes with /v2 and carries the ticker in the path;
// we key off the trailing {package}/{category}[/{sub}] structure.
func schemaForOperation(operation string) string {
	parts := strings.SplitN(operation, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	seg := strings.Split(strings.Trim(strings.TrimPrefix(parts[1], "/v2"), "/"), "/")
	if len(seg) == 1 && seg[0] == "tickers" {
		return "TickersResponse"
	}
	if len(seg) == 2 && seg[0] == "tickers" && seg[1] == "quant" {
		return "QuantTickersResponse"
	}
	// market-data: {ticker}/{package}/{category}[/{sub}]
	if len(seg) < 3 {
		return ""
	}
	pkg, category, sub := seg[1], seg[2], ""
	if len(seg) >= 4 {
		sub = seg[3]
	}
	switch pkg {
	case "classic":
		switch sub {
		case "":
			return "GexData"
		case "majors":
			return "GexMajorsData"
		case "maxchange":
			return "GexMaxChangeData"
		}
	case "state":
		switch sub {
		case "":
			if strings.HasPrefix(category, "gex_") {
				return "GexData"
			}
			return "GreekProfileData"
		case "majors":
			return "GexMajorsData"
		case "maxchange":
			return "GexMaxChangeData"
		}
	case "orderflow":
		return "OrderflowData"
	}
	return ""
}

// TestServedResponseFieldsMatchLiveProbe asserts that, for every served endpoint
// captured in the newest live probe, the faker's OpenAPI response schema declares
// exactly the fields the live API returns. Oracle = the live probe (not the
// upstream schema, which is too generic for state/orderflow — see AUDIT-*.md).
func TestServedResponseFieldsMatchLiveProbe(t *testing.T) {
	probes, err := filepath.Glob("live-probes/*.json")
	if err != nil || len(probes) == 0 {
		t.Fatalf("no live probe fixtures: %v", err)
	}
	sort.Strings(probes)
	newest := probes[len(probes)-1] // filenames are dates; last = most recent

	var probe liveProbeDoc
	readJSON(t, newest, &probe)

	var faker schemaDoc
	readYAML(t, "../api/openapi.yaml", &faker)

	checked := 0
	for _, p := range probe.Probes {
		if p.Status != 200 || len(p.Fields) == 0 {
			continue
		}
		schemaName := schemaForOperation(p.Operation)
		if schemaName == "" {
			continue // endpoint the faker does not serve
		}
		schema, ok := faker.Components.Schemas[schemaName]
		if !ok {
			t.Errorf("%s: schema %q referenced for %s is missing from the faker spec", newest, schemaName, p.Operation)
			continue
		}
		want := append([]string(nil), p.Fields...)
		got := make([]string, 0, len(schema.Properties))
		for name := range schema.Properties {
			got = append(got, name)
		}
		sort.Strings(want)
		sort.Strings(got)
		if strings.Join(want, ",") != strings.Join(got, ",") {
			t.Errorf("%s (%s -> %s): response fields drifted from the live API\n  live:  %v\n  faker: %v",
				p.Operation, newest, schemaName, want, got)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no served endpoints were checked against the live probe")
	}
	t.Logf("verified %d served endpoints against %s", checked, newest)
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
