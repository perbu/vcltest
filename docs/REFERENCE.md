# VCLTest YAML Reference

This document describes the YAML test specification format for VCLTest.

For IDE autocompletion, add this to your YAML files:

```yaml
# yaml-language-server: $schema=../schema.json
```

## Test Structure

A test file can contain one or more test specifications separated by `---`. Each test is either a single-request test or
a scenario test - multistep temporal test.

```yaml
# Single-request test
name: "My test"
request:
  url: /path
backends:
  default: { status: 200 }
expectations:
  response: { status: 200 }

---

# Scenario test
name: "Cache expiry test"
scenario:
  - at: 0s
    request: { url: /path }
    expectations: { response: { status: 200 } }
  - at: 5m
    request: { url: /path }
    expectations: { cache: { hit: false } }
```

---

## Top-Level Fields

| Field          | Type   | Required | Description                           |
|----------------|--------|----------|---------------------------------------|
| `name`         | string | Yes      | Name of the test case                 |
| `request`      | object | No*      | HTTP request specification            |
| `backends`     | object | No       | Named backend response configurations |
| `expectations` | object | No*      | Expected results                      |
| `scenario`     | array  | No*      | Multi-step temporal test              |

*Either `request`/`expectations` OR `scenario` must be provided, not both.

---

## Request

Defines the HTTP request to send through Varnish.

```yaml
request:
  method: POST         # Optional, default: GET
  url: /api/users      # Required
  headers: # Optional
    User-Agent: "My little app"
  body: '{"key": "value"}'  # Optional
```

| Field     | Type   | Required | Description                                                             |
|-----------|--------|----------|-------------------------------------------------------------------------|
| `method`  | string | No       | HTTP method: GET, POST or any other string, the string is not validated |
| `url`     | string | Yes      | URL path to request                                                     |
| `headers` | object | No       | Request headers (string key-value pairs)                                |
| `body`    | string | No       | Request body content                                                    |

---

## Backends

Configures mock backend responses. Backend names must match the backend declarations in your VCL. Note that
paths are optional. In a lot of tests you just want something to respond, so you can omit the path.

```yaml
backends:
  default:
    status: 200
    headers:
      Content-Type: application/json
      Cache-Control: max-age=300
    body: '{"result": "ok"}'

  api:
    status: 503
    failure_mode: failed  # Simulate connection failure
```

### Backend Fields

| Field          | Type    | Required | Description                                                        |
|----------------|---------|----------|--------------------------------------------------------------------|
| `status`       | integer | No       | HTTP status code (100-599), default: 200                           |
| `headers`      | object  | No       | Response headers                                                   |
| `body`         | string  | No       | Response body                                                      |
| `failure_mode` | string  | No       | Failure simulation: `failed` (connection reset) or `frozen` (hang) |
| `routes`       | object  | No       | Path-based response routing                                        |

### Path-Based Routing

For backends that need different responses based on URL path. Note that vcltest will fall back to the default
backend if no route matches. 

```yaml
backends:
  api:
    status: 404
    body: 'Object not found'
    routes:
      /users:
        status: 200
        body: '[{"id": 1}]'
      /health:
        status: 200
        body: 'OK'
      /error:
        status: 500
        body: 'Internal error'
```

Each route supports the same fields as a backend (`status`, `headers`, `body`, `failure_mode`).

---

## Expectations

These are the assertions part of your test. Defines what to assert about the response.

```yaml
expectations:
  response:
    status: 200
    headers:
      X-Cache: HIT
    body_contains: "success"

  backend:
    calls: 1
    used: api

  cache:
    hit: true
    age_lt: 60

  cookies:
    session_id: "abc123"
```

### Response Expectations

| Field           | Type    | Required | Description                        |
|-----------------|---------|----------|------------------------------------|
| `status`        | integer | Yes      | Expected HTTP status code          |
| `headers`       | object  | No       | Expected headers (exact match)     |
| `body_contains` | string  | No       | Substring that must appear in body |

### Backend Expectations

| Field      | Type    | Required | Description                           |
|------------|---------|----------|---------------------------------------|
| `calls`    | integer | No       | Expected total backend calls          |
| `used`     | string  | No       | Name of backend that should be called |
| `backends` | object  | No       | Per-backend call count expectations   |

Per-backend call counts. vcltest will watch the varnishlog for BackendOpen and count the number of times each backend
is called. This provides a quick and easy way to verify if a backend was called.

```yaml
expectations:
  backend:
    backends:
      api:
        calls: 2
      cache:
        calls: 0
```

### Cache Expectations

`hit` will look at X-Varnish header. One number means `hit` is `false` and two numbers means `hit` is `true`. If for
some reason this header is missing, we look at the Age header.

| Field    | Type    | Required | Description                              |
|----------|---------|----------|------------------------------------------|
| `hit`    | boolean | No       | `true` = cache hit, `false` = cache miss |
| `age_gt` | integer | No       | Age header must be > N seconds           |
| `age_lt` | integer | No       | Age header must be < N seconds           |

### Cookie Expectations

The HTTP client has a cookie jar and when it encounters a Set-Cookie header, it stores it in the cookie jar. So, if your
VCL logic relies on cookies, you can assert they are present in the cookie jar.

```yaml
expectations:
  cookies:
    session: "value"
    tracking_id: "12345"
```

Verifies cookies present in the cookie jar after the request.

---

## Scenario Tests

Scenario tests execute multiple steps with time manipulation, useful for testing cache TTLs, grace periods, and
time-dependent VCL logic. vcltest will use libfaketime to manipulate time. note that time cannot travel backwards, or
varnish will likely get confused and panic, so if you have many years of TTLs and many tests, you might travel far into
the future and see how Varnish deals with `Year 2038 problem`.

```yaml
name: "Cache TTL expiry"
backends:
  default:
    status: 200
    headers:
      Cache-Control: max-age=300
    body: "cached content"

scenario:
  - at: 0s
    request:
      url: /cached
    expectations:
      response:
        status: 200
      cache:
        hit: false

  - at: 30s
    request:
      url: /cached
    expectations:
      response:
        status: 200
      cache:
        hit: true
        age_gt: 25

  - at: 6m
    request:
      url: /cached
    expectations:
      cache:
        hit: false  # TTL expired
```

### Scenario Step Fields

| Field          | Type   | Required | Description                                          |
|----------------|--------|----------|------------------------------------------------------|
| `at`           | string | Yes      | Time offset from test start: `0s`, `30s`, `2m`, `1h` |
| `request`      | object | No       | HTTP request (same format as top-level)              |
| `backends`     | object | No       | Backend overrides for this step                      |
| `expectations` | object | Yes      | Assertions for this step                             |

Time format: `<number><unit>` where unit is `s` (seconds), `m` (minutes), or `h` (hours).

### Overriding Backends Per Step

Scenario steps can override backend behavior. So a backend can be set to fail at a certain point in the scenario, or
change content.

```yaml
scenario:
  - at: 0s
    request: { url: /api }
    backends:
      api: { status: 200, body: "ok" }
    expectations:
      response: { status: 200 }

  - at: 30s
    request: { url: /api }
    backends:
      api: { status: 503, failure_mode: failed }
    expectations:
      response: { status: 503 }  # Or whatever your VCL returns on backend failure
```

---

## VCL Resolution

VCLTest does not use a `vcl` field in the YAML. Instead:

1. **CLI flag**: `vcltest -vcl production.vcl tests.yaml`
2. **Auto-detection**: If no flag, looks for `tests.vcl` when running `tests.yaml`

---

## Complete Example

```yaml
name: "API caching with authentication bypass"

request:
  method: GET
  url: /api/products
  headers:
    Accept: application/json

backends:
  api:
    status: 200
    headers:
      Content-Type: application/json
      Cache-Control: max-age=60
    body: '[{"id": 1, "name": "Widget"}]'

expectations:
  response:
    status: 200
    headers:
      Content-Type: application/json
    body_contains: "Widget"
  backend:
    used: api
    calls: 1
  cache:
    hit: false

---

name: "Authenticated requests bypass cache"

request:
  method: GET
  url: /api/products
  headers:
    Authorization: "Bearer secret"

backends:
  api:
    status: 200
    body: '{"user": "specific"}'

expectations:
  response:
    status: 200
  cache:
    hit: false  # Auth requests should not be cached
```

---

## Schema Reference

For the complete machine-readable specification, see [`schema.json`](../schema.json).
