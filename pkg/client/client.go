package client

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/perbu/vcltest/pkg/testspec"
)

// Response represents an HTTP response
type Response struct {
	Status  int
	Headers http.Header
	Body    string
}

// MakeRequest makes an HTTP request to Varnish according to the test spec.
// If httpClient is nil, a default client is created (no cookie persistence).
// Pass a client with a CookieJar for cookie persistence across requests.
func MakeRequest(httpClient *http.Client, varnishURL string, req testspec.RequestSpec) (*Response, error) {
	// Build full URL
	url := varnishURL + req.URL

	// Create HTTP request
	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequest(req.Method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Add headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Use provided client or create default
	// Important: Don't follow redirects automatically - we want to test the redirect response itself
	// Also disable keep-alive to ensure connections are closed after each request,
	// which allows varnish to shut down cleanly.
	if httpClient == nil {
		httpClient = &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	return &Response{
		Status:  resp.StatusCode,
		Headers: resp.Header,
		Body:    string(bodyBytes),
	}, nil
}
