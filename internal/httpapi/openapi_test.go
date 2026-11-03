package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"

	"github.com/app-torq/torq/internal/testutil"
)

// The contract tests check openapi.yaml against the code (research M9, M10):
// it must be valid OpenAPI 3.1, list exactly the registered routes with their
// access levels, and describe every JSON response the HTTP tests produce
// (validated in client.do).

const (
	contractFile = "../../openapi.yaml"
	contractURL  = "https://torq.invalid/openapi.json"
	oasSchemaURL = "https://spec.openapis.org/oas/3.1/schema/2022-10-07"
)

var loadedContract = sync.OnceValues(loadContract)

type contractOp struct {
	method, path string
	re           *regexp.Regexp
	op           map[string]any
	pointer      string // JSON pointer of the operation
}

type contractDoc struct {
	doc      map[string]any
	ops      []contractOp
	compiler *jsonschema.Compiler

	mu      sync.Mutex
	schemas map[string]*jsonschema.Schema
}

func loadContract() (*contractDoc, error) {
	raw, err := os.ReadFile(contractFile)
	if err != nil {
		return nil, err
	}
	var y any
	if err := yaml.Unmarshal(raw, &y); err != nil {
		return nil, err
	}
	b, err := json.Marshal(y)
	if err != nil {
		return nil, err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	m, ok := doc.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: not an object", contractFile)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	if err := c.AddResource(contractURL, doc); err != nil {
		return nil, err
	}
	cd := &contractDoc{doc: m, compiler: c, schemas: map[string]*jsonschema.Schema{}}
	paths, _ := m["paths"].(map[string]any)
	for p, item := range paths {
		methods, _ := item.(map[string]any)
		for method, op := range methods {
			opm, ok := op.(map[string]any)
			if !ok {
				continue
			}
			cd.ops = append(cd.ops, contractOp{
				method: strings.ToUpper(method), path: p, op: opm,
				re:      pathPattern(p),
				pointer: "/paths/" + escapePointer(p) + "/" + method,
			})
		}
	}
	// Literal paths win over templated ones (/me/sessions vs /me/sessions/{id}).
	slices.SortFunc(cd.ops, func(a, b contractOp) int { return strings.Count(a.path, "{") - strings.Count(b.path, "{") })
	return cd, nil
}

// pathPattern matches request paths against a templated contract path.
func pathPattern(p string) *regexp.Regexp {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			parts[i] = "[^/]+"
		} else {
			parts[i] = regexp.QuoteMeta(part)
		}
	}
	return regexp.MustCompile("^" + strings.Join(parts, "/") + "$")
}

func escapePointer(s string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(s)
}

func contract(t *testing.T) *contractDoc {
	t.Helper()
	cd, err := loadedContract()
	if err != nil {
		t.Fatalf("load %s: %v", contractFile, err)
	}
	return cd
}

// find returns the operation for a request path below /api/v1.
func (cd *contractDoc) find(method, path string) (contractOp, bool) {
	for _, o := range cd.ops {
		if o.method == method && o.re.MatchString(path) {
			return o, true
		}
	}
	return contractOp{}, false
}

// responseSchema compiles the JSON schema for one operation and status,
// falling back to the "default" response, and resolves response $refs.
func (cd *contractDoc) responseSchema(o contractOp, status int) (*jsonschema.Schema, error) {
	responses, _ := o.op["responses"].(map[string]any)
	key := strconv.Itoa(status)
	resp, ok := responses[key]
	if !ok {
		key = "default"
		if resp, ok = responses[key]; !ok {
			return nil, fmt.Errorf("no response %d or default", status)
		}
	}
	pointer := o.pointer + "/responses/" + key
	if r, ok := resp.(map[string]any)["$ref"].(string); ok {
		pointer = strings.TrimPrefix(r, "#")
		resp = cd.lookup(pointer)
	}
	content, _ := resp.(map[string]any)["content"].(map[string]any)
	if _, ok := content["application/json"]; !ok {
		return nil, fmt.Errorf("response %s has no application/json content", key)
	}
	loc := contractURL + "#" + pointer + "/content/application~1json/schema"
	cd.mu.Lock()
	defer cd.mu.Unlock()
	if s, ok := cd.schemas[loc]; ok {
		return s, nil
	}
	s, err := cd.compiler.Compile(loc)
	if err != nil {
		return nil, err
	}
	cd.schemas[loc] = s
	return s, nil
}

func (cd *contractDoc) lookup(pointer string) any {
	var v any = cd.doc
	for _, part := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		part = strings.NewReplacer("~1", "/", "~0", "~").Replace(part)
		m, _ := v.(map[string]any)
		v = m[part]
	}
	return v
}

// checkResponse validates a JSON response body against the contract. Responses
// without a body or with another content type (downloads, exports) are skipped.
func (cd *contractDoc) checkResponse(method, path string, status int, contentType string, body []byte) error {
	path, _, _ = strings.Cut(strings.TrimPrefix(path, "/api/v1"), "?")
	o, ok := cd.find(method, path)
	if !ok {
		return fmt.Errorf("not in openapi.yaml")
	}
	if len(body) == 0 || !strings.HasPrefix(contentType, "application/json") {
		return nil
	}
	if o.path == "/vehicles/{id}/history" && status == http.StatusOK {
		return nil // JSON export: validated by TestHistoryMatchesContract
	}
	s, err := cd.responseSchema(o, status)
	if err != nil {
		return err
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		return err
	}
	return s.Validate(v)
}

func TestOpenAPIIsValid(t *testing.T) {
	cd := contract(t)
	raw, err := os.ReadFile("testdata/openapi-3.1-schema.json")
	if err != nil {
		t.Fatal(err)
	}
	oas, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(oasSchemaURL, oas); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(oasSchemaURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(any(cd.doc)); err != nil {
		t.Fatalf("openapi.yaml is not valid OpenAPI 3.1: %v", err)
	}
	// Every schema in the document must compile, so $refs resolve.
	for name := range cd.doc["components"].(map[string]any)["schemas"].(map[string]any) {
		if _, err := cd.compiler.Compile(contractURL + "#/components/schemas/" + name); err != nil {
			t.Errorf("schema %s: %v", name, err)
		}
	}
}

func TestOpenAPICoversRoutes(t *testing.T) {
	cd := contract(t)
	s := NewServer(Deps{
		Config: newEnv(t).cfg, DB: testutil.NewDB(t), Clock: testutil.FixedClock(t, "2026-10-30"),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Version: "test", Dist: fstest.MapFS{"index.html": {}},
	})
	norm := regexp.MustCompile(`\{[^}]+\}`)
	key := func(method, path string) string { return method + " " + norm.ReplaceAllString(path, "{}") }

	registered := map[string]access{}
	for _, p := range s.patterns {
		method, path, _ := strings.Cut(p, " ")
		registered[key(method, strings.TrimPrefix(path, "/api/v1"))] = s.routeAccess[p]
	}
	documented := map[string]string{}
	for _, o := range cd.ops {
		level, _ := o.op["x-torq-access"].(string)
		documented[key(o.method, o.path)] = level
	}
	names := map[access]string{public: "public", member: "member", admin: "admin"}
	for k, level := range registered {
		doc, ok := documented[k]
		switch {
		case !ok:
			t.Errorf("%s is registered but missing from openapi.yaml", k)
		case doc != names[level]:
			t.Errorf("%s: openapi.yaml says x-torq-access %q, the route is %q", k, doc, names[level])
		}
	}
	for k := range documented {
		if _, ok := registered[k]; !ok {
			t.Errorf("%s is in openapi.yaml but not registered", k)
		}
	}
	if v := cd.doc["info"].(map[string]any)["version"]; v != strconv.Itoa(APIVersion) {
		t.Errorf("info.version = %v, APIVersion = %d", v, APIVersion)
	}
}

// The JSON history export is a download, so client.do skips it; this checks
// it against its schema directly.
func TestHistoryMatchesContract(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	var v struct {
		Vehicle struct {
			ID int64 `json:"id"`
		} `json:"vehicle"`
	}
	c.call(http.MethodPost, "/vehicles", map[string]any{"make": "Mercedes-Benz", "model": "C 220 CDI", "current_odometer_km": 215000}, http.StatusCreated, &v)
	rec := c.do(http.MethodGet, fmt.Sprintf("/vehicles/%d/history?download=0", v.Vehicle.ID), nil)
	cd := contract(t)
	o, _ := cd.find(http.MethodGet, "/vehicles/{id}/history")
	s, err := cd.responseSchema(o, http.StatusOK)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(doc); err != nil {
		t.Fatalf("history export: %v", err)
	}
}
