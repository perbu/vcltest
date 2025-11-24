package testspec

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBackendExpectations_UnmarshalYAML_SimpleString(t *testing.T) {
	yamlStr := `backend: "api_server"`

	var spec struct {
		Backend *BackendExpectations `yaml:"backend"`
	}

	err := yaml.Unmarshal([]byte(yamlStr), &spec)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if spec.Backend == nil {
		t.Fatal("backend is nil")
	}

	if spec.Backend.Name != "api_server" {
		t.Errorf("expected Name to be 'api_server', got %q", spec.Backend.Name)
	}
}

func TestBackendExpectations_UnmarshalYAML_ObjectWithUsed(t *testing.T) {
	yamlStr := `backend:
  used: "api_server"
  calls: 2`

	var spec struct {
		Backend *BackendExpectations `yaml:"backend"`
	}

	err := yaml.Unmarshal([]byte(yamlStr), &spec)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if spec.Backend == nil {
		t.Fatal("backend is nil")
	}

	if spec.Backend.Used != "api_server" {
		t.Errorf("expected Used to be 'api_server', got %q", spec.Backend.Used)
	}

	if spec.Backend.Calls == nil {
		t.Fatal("Calls is nil")
	}

	if *spec.Backend.Calls != 2 {
		t.Errorf("expected Calls to be 2, got %d", *spec.Backend.Calls)
	}
}

func TestBackendExpectations_UnmarshalYAML_ObjectWithJustCalls(t *testing.T) {
	yamlStr := `backend:
  calls: 0`

	var spec struct {
		Backend *BackendExpectations `yaml:"backend"`
	}

	err := yaml.Unmarshal([]byte(yamlStr), &spec)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if spec.Backend == nil {
		t.Fatal("backend is nil")
	}

	if spec.Backend.Calls == nil {
		t.Fatal("Calls is nil")
	}

	if *spec.Backend.Calls != 0 {
		t.Errorf("expected Calls to be 0, got %d", *spec.Backend.Calls)
	}
}

func TestBackendExpectations_UnmarshalYAML_PerBackend(t *testing.T) {
	yamlStr := `backend:
  backends:
    api_server:
      calls: 1
    web_server:
      calls: 0`

	var spec struct {
		Backend *BackendExpectations `yaml:"backend"`
	}

	err := yaml.Unmarshal([]byte(yamlStr), &spec)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if spec.Backend == nil {
		t.Fatal("backend is nil")
	}

	if len(spec.Backend.PerBackend) != 2 {
		t.Fatalf("expected 2 backends in PerBackend, got %d", len(spec.Backend.PerBackend))
	}

	apiServer, ok := spec.Backend.PerBackend["api_server"]
	if !ok {
		t.Fatal("api_server not found in PerBackend")
	}
	if apiServer.Calls != 1 {
		t.Errorf("expected api_server calls to be 1, got %d", apiServer.Calls)
	}

	webServer, ok := spec.Backend.PerBackend["web_server"]
	if !ok {
		t.Fatal("web_server not found in PerBackend")
	}
	if webServer.Calls != 0 {
		t.Errorf("expected web_server calls to be 0, got %d", webServer.Calls)
	}
}

func TestBackendExpectations_CompleteExpectationsSpec(t *testing.T) {
	yamlStr := `response:
  status: 200
backend: "api_server"
cache:
  hit: true`

	var spec ExpectationsSpec

	err := yaml.Unmarshal([]byte(yamlStr), &spec)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if spec.Response.Status != 200 {
		t.Errorf("expected status 200, got %d", spec.Response.Status)
	}

	if spec.Backend == nil {
		t.Fatal("backend is nil")
	}

	if spec.Backend.Name != "api_server" {
		t.Errorf("expected backend name to be 'api_server', got %q", spec.Backend.Name)
	}

	if spec.Cache == nil {
		t.Fatal("cache is nil")
	}

	if spec.Cache.Hit == nil || !*spec.Cache.Hit {
		t.Error("expected cache hit to be true")
	}
}
