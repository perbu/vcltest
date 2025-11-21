package testspec

// TestSpec represents a single test case
type TestSpec struct {
	Name    string      `yaml:"name"`
	VCL     string      `yaml:"vcl"`
	Request RequestSpec `yaml:"request"`
	Backend BackendSpec `yaml:"backend"`
	Expect  ExpectSpec  `yaml:"expect"`
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
}

// ApplyDefaults sets default values for optional fields
func (t *TestSpec) ApplyDefaults() {
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
}
