package testspec

// TestSpec represents a single test case
type TestSpec struct {
	Name         string                 `yaml:"name"`
	Request      RequestSpec            `yaml:"request,omitempty"`      // Optional: for single-request tests
	Backend      BackendSpec            `yaml:"backend,omitempty"`      // Optional: for single-request tests (legacy)
	Backends     map[string]BackendSpec `yaml:"backends,omitempty"`     // Optional: for multi-backend tests
	Expectations ExpectationsSpec       `yaml:"expectations,omitempty"` // Optional: for single-request tests
	Scenario     []ScenarioStep         `yaml:"scenario,omitempty"`     // Optional: for multi-step temporal tests
}

// ScenarioStep represents a single step in a temporal test scenario
type ScenarioStep struct {
	At           string           `yaml:"at"`                // Time offset from test start (e.g., "0s", "30s", "2m")
	Request      RequestSpec      `yaml:"request,omitempty"` // HTTP request to make
	Backend      BackendSpec      `yaml:"backend,omitempty"` // Mock backend response (if different from previous)
	Expectations ExpectationsSpec `yaml:"expectations"`      // Expectations for this step
}

// RequestSpec defines the HTTP request to make
type RequestSpec struct {
	Method  string            `yaml:"method,omitempty"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty"`
}

// BackendSpec defines the mock backend response
type BackendSpec struct {
	Status  int               `yaml:"status,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty"`
}

// ExpectationsSpec defines all test expectations (nested structure)
type ExpectationsSpec struct {
	Response ResponseExpectations `yaml:"response"`
	Backend  *BackendExpectations `yaml:"backend,omitempty"`
	Cache    *CacheExpectations   `yaml:"cache,omitempty"`
}

// ResponseExpectations validates what the client receives from Varnish
type ResponseExpectations struct {
	Status       int               `yaml:"status"`
	Headers      map[string]string `yaml:"headers,omitempty"`
	BodyContains string            `yaml:"body_contains,omitempty"`
}

// BackendExpectations validates backend interaction
// Supports multiple formats:
// 1. Simple string: backend: "api_server"
// 2. Object with used/calls: backend: {used: "api_server", calls: 2}
// 3. Per-backend map: backends: {api_server: {calls: 1}}
type BackendExpectations struct {
	// Simple format (populated by UnmarshalYAML when value is a string)
	Name string `yaml:"-"` // Not directly in YAML, set by custom unmarshaler

	// Object format
	Calls *int   `yaml:"calls,omitempty"`
	Used  string `yaml:"used,omitempty"`

	// Per-backend map format
	PerBackend map[string]BackendCallExpectation `yaml:"backends,omitempty"`
}

// BackendCallExpectation defines expected calls for a specific backend
type BackendCallExpectation struct {
	Calls int `yaml:"calls"`
}

// UnmarshalYAML implements custom unmarshaling to support simple string format
func (b *BackendExpectations) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as a string first (simple format)
	var simple string
	if err := unmarshal(&simple); err == nil {
		b.Name = simple
		return nil
	}

	// If that fails, unmarshal as an object
	type rawBackendExpectations BackendExpectations
	raw := (*rawBackendExpectations)(b)
	return unmarshal(raw)
}

// CacheExpectations validates cache-specific behavior
type CacheExpectations struct {
	Hit   *bool `yaml:"hit,omitempty"`
	AgeGt *int  `yaml:"age_gt,omitempty"`
	AgeLt *int  `yaml:"age_lt,omitempty"`
	Stale *bool `yaml:"stale,omitempty"`
}

// ApplyDefaults sets default values for optional fields
func (t *TestSpec) ApplyDefaults() {
	// For single-request tests
	if len(t.Scenario) == 0 {
		// Request defaults
		if t.Request.Method == "" {
			t.Request.Method = "GET"
		}

		// Backend defaults
		if t.Backend.Status == 0 {
			t.Backend.Status = 200
		}
		if t.Backend.Headers == nil {
			t.Backend.Headers = make(map[string]string)
		}

		// Response expectations default
		if t.Expectations.Response.Status == 0 {
			t.Expectations.Response.Status = 200
		}
	} else {
		// For scenario-based tests, apply defaults to each step
		for i := range t.Scenario {
			if t.Scenario[i].Request.Method == "" {
				t.Scenario[i].Request.Method = "GET"
			}
			if t.Scenario[i].Backend.Status == 0 && t.Scenario[i].Backend.Headers != nil {
				t.Scenario[i].Backend.Status = 200
			}
			if t.Scenario[i].Backend.Headers == nil {
				t.Scenario[i].Backend.Headers = make(map[string]string)
			}

			// Response expectations default
			if t.Scenario[i].Expectations.Response.Status == 0 {
				t.Scenario[i].Expectations.Response.Status = 200
			}
		}
	}
}

// IsScenario returns true if this is a scenario-based test
func (t *TestSpec) IsScenario() bool {
	return len(t.Scenario) > 0
}
