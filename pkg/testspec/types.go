package testspec

// TestSpec represents a single test case
type TestSpec struct {
	Name         string                 `yaml:"name" json:"name" jsonschema:"required,description=Name of the test case"`
	Request      RequestSpec            `yaml:"request,omitempty" json:"request,omitempty" jsonschema:"description=HTTP request specification for single-request tests"`
	Backends     map[string]BackendSpec `yaml:"backends,omitempty" json:"backends,omitempty" jsonschema:"description=Named backend response specifications"`
	Expectations ExpectationsSpec       `yaml:"expectations,omitempty" json:"expectations,omitempty" jsonschema:"description=Test expectations for single-request tests"`
	Scenario     []ScenarioStep         `yaml:"scenario,omitempty" json:"scenario,omitempty" jsonschema:"description=Multi-step temporal test scenario"`
}

// ScenarioStep represents a single step in a temporal test scenario
type ScenarioStep struct {
	At           string                 `yaml:"at" json:"at" jsonschema:"required,description=Time offset from test start (e.g. '0s' '30s' '2m'),pattern=^[0-9]+(s|m|h)$"`
	Request      RequestSpec            `yaml:"request,omitempty" json:"request,omitempty" jsonschema:"description=HTTP request to make at this step"`
	Backends     map[string]BackendSpec `yaml:"backends,omitempty" json:"backends,omitempty" jsonschema:"description=Backend response overrides for this step"`
	Expectations ExpectationsSpec       `yaml:"expectations" json:"expectations" jsonschema:"required,description=Test expectations for this step"`
}

// RequestSpec defines the HTTP request to make
type RequestSpec struct {
	Method  string            `yaml:"method,omitempty" json:"method,omitempty" jsonschema:"description=HTTP method (default: GET),enum=GET,enum=POST,enum=PUT,enum=DELETE,enum=HEAD,enum=PATCH,enum=OPTIONS"`
	URL     string            `yaml:"url" json:"url" jsonschema:"required,description=URL path to request (e.g. '/api/users')"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty" jsonschema:"description=HTTP request headers"`
	Body    string            `yaml:"body,omitempty" json:"body,omitempty" jsonschema:"description=Request body content"`
}

// RouteSpec defines response for a specific URL path
type RouteSpec struct {
	Status      int               `yaml:"status,omitempty" json:"status,omitempty" jsonschema:"description=HTTP status code (default: 404),minimum=100,maximum=599"`
	Headers     map[string]string `yaml:"headers,omitempty" json:"headers,omitempty" jsonschema:"description=HTTP response headers"`
	Body        string            `yaml:"body,omitempty" json:"body,omitempty" jsonschema:"description=Response body content"`
	FailureMode string            `yaml:"failure_mode,omitempty" json:"failure_mode,omitempty" jsonschema:"description=Backend failure simulation (failed=connection reset, frozen=never responds),enum=failed,enum=frozen"`
	EchoRequest bool              `yaml:"echo_request,omitempty" json:"echo_request,omitempty" jsonschema:"description=Return the incoming request as JSON (for testing VCL request transformations)"`
}

// BackendSpec defines the mock backend response
type BackendSpec struct {
	Status      int                  `yaml:"status,omitempty" json:"status,omitempty" jsonschema:"description=HTTP status code (default: 404),minimum=100,maximum=599"`
	Headers     map[string]string    `yaml:"headers,omitempty" json:"headers,omitempty" jsonschema:"description=HTTP response headers from backend"`
	Body        string               `yaml:"body,omitempty" json:"body,omitempty" jsonschema:"description=Response body content from backend"`
	FailureMode string               `yaml:"failure_mode,omitempty" json:"failure_mode,omitempty" jsonschema:"description=Backend failure simulation (failed=connection reset, frozen=never responds),enum=failed,enum=frozen"`
	Routes      map[string]RouteSpec `yaml:"routes,omitempty" json:"routes,omitempty" jsonschema:"description=URL path to response mapping for path-based routing"`
	EchoRequest bool                 `yaml:"echo_request,omitempty" json:"echo_request,omitempty" jsonschema:"description=Return the incoming request as JSON (for testing VCL request transformations)"`
}

// ExpectationsSpec defines all test expectations (nested structure)
type ExpectationsSpec struct {
	Response ResponseExpectations `yaml:"response" json:"response" jsonschema:"required,description=Expected HTTP response from Varnish"`
	Backend  *BackendExpectations `yaml:"backend,omitempty" json:"backend,omitempty" jsonschema:"description=Expected backend interaction"`
	Cache    *CacheExpectations   `yaml:"cache,omitempty" json:"cache,omitempty" jsonschema:"description=Expected cache behavior"`
	Cookies  map[string]string    `yaml:"cookies,omitempty" json:"cookies,omitempty" jsonschema:"description=Expected cookies in jar (name: value)"`
}

// ResponseExpectations validates what the client receives from Varnish
type ResponseExpectations struct {
	Status       int               `yaml:"status" json:"status" jsonschema:"required,description=Expected HTTP status code,minimum=100,maximum=599"`
	Headers      map[string]string `yaml:"headers,omitempty" json:"headers,omitempty" jsonschema:"description=Expected HTTP response headers"`
	BodyContains string            `yaml:"body_contains,omitempty" json:"body_contains,omitempty" jsonschema:"description=Substring that must appear in response body"`
}

// BackendExpectations validates backend interaction
// Supports multiple formats:
// 1. Simple string: backend: "api_server"
// 2. Object with used/calls: backend: {used: "api_server", calls: 2}
// 3. Per-backend map: backends: {api_server: {calls: 1}}
type BackendExpectations struct {
	// Simple format (populated by UnmarshalYAML when value is a string)
	Name string `yaml:"-" json:"-" jsonschema:"-"` // Not directly in YAML, set by custom unmarshaler

	// Object format
	Calls *int   `yaml:"calls,omitempty" json:"calls,omitempty" jsonschema:"description=Expected number of backend calls"`
	Used  string `yaml:"used,omitempty" json:"used,omitempty" jsonschema:"description=Name of backend that should be used"`

	// Per-backend map format
	PerBackend map[string]BackendCallExpectation `yaml:"backends,omitempty" json:"backends,omitempty" jsonschema:"description=Per-backend call count expectations"`
}

// BackendCallExpectation defines expected calls for a specific backend
type BackendCallExpectation struct {
	Calls int `yaml:"calls" json:"calls" jsonschema:"required,description=Expected number of calls to this backend"`
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
	Hit   *bool `yaml:"hit,omitempty" json:"hit,omitempty" jsonschema:"description=Whether response should be a cache hit (true) or miss (false)"`
	AgeGt *int  `yaml:"age_gt,omitempty" json:"age_gt,omitempty" jsonschema:"description=Age header must be greater than this value in seconds"`
	AgeLt *int  `yaml:"age_lt,omitempty" json:"age_lt,omitempty" jsonschema:"description=Age header must be less than this value in seconds"`
}

// ApplyDefaults sets default values for optional fields
func (t *TestSpec) ApplyDefaults() {
	// For single-request tests
	if len(t.Scenario) == 0 {
		// Request defaults
		if t.Request.Method == "" {
			t.Request.Method = "GET"
		}

		// Backend defaults - apply to all backends in the map
		for name, spec := range t.Backends {
			if spec.Status == 0 {
				spec.Status = 404
			}
			if spec.Headers == nil {
				spec.Headers = make(map[string]string)
			}
			t.Backends[name] = spec
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

			// Apply defaults to step-level backend overrides
			for name, spec := range t.Scenario[i].Backends {
				if spec.Status == 0 {
					spec.Status = 404
				}
				if spec.Headers == nil {
					spec.Headers = make(map[string]string)
				}
				t.Scenario[i].Backends[name] = spec
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
