package testspec

// TestSpec represents a single test case
type TestSpec struct {
	Name     string         `yaml:"name"`
	VCL      string         `yaml:"vcl"`
	Request  RequestSpec    `yaml:"request,omitempty"`  // Optional: for single-request tests
	Backend  BackendSpec    `yaml:"backend,omitempty"`  // Optional: for single-request tests
	Expect   ExpectSpec     `yaml:"expect,omitempty"`   // Optional: for single-request tests
	Scenario []ScenarioStep `yaml:"scenario,omitempty"` // Optional: for multi-step temporal tests
}

// ScenarioStep represents a single step in a temporal test scenario
type ScenarioStep struct {
	At      string      `yaml:"at"`                // Time offset from test start (e.g., "0s", "30s", "2m")
	Request RequestSpec `yaml:"request,omitempty"` // HTTP request to make
	Backend BackendSpec `yaml:"backend,omitempty"` // Mock backend response (if different from previous)
	Expect  ExpectSpec  `yaml:"expect"`            // Expectations for this step
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

// ExpectSpec defines test expectations
type ExpectSpec struct {
	Status       int               `yaml:"status"`
	BackendCalls *int              `yaml:"backend_calls,omitempty"`
	Headers      map[string]string `yaml:"headers,omitempty"`
	BodyContains string            `yaml:"body_contains,omitempty"`
	Cached       *bool             `yaml:"cached,omitempty"` // Check if response was cached
	AgeGt        *int              `yaml:"age_gt,omitempty"` // Age header > N seconds
	AgeLt        *int              `yaml:"age_lt,omitempty"` // Age header < N seconds
	Stale        *bool             `yaml:"stale,omitempty"`  // Check if stale content served
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
		}
	}
}

// IsScenario returns true if this is a scenario-based test
func (t *TestSpec) IsScenario() bool {
	return len(t.Scenario) > 0
}
