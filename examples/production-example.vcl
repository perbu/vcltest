vcl 4.1;

# Production-like VCL with real hostnames
# This demonstrates the new AST-based backend replacement
# No need for __BACKEND_HOST__ placeholders!

backend api_production {
    .host = "api-prod.example.com";
    .port = "443";
    .connect_timeout = 5s;
    .first_byte_timeout = 60s;
}

backend web_production {
    .host = "web-prod.example.com";
    .port = "443";
    .connect_timeout = 3s;
    .first_byte_timeout = 30s;
}

backend legacy_service {
    .host = "legacy.internal.example.com";
    .port = "8080";
}

sub vcl_recv {
    # Route API requests to api_production
    if (req.url ~ "^/api/") {
        set req.backend_hint = api_production;
        return (pass);
    }

    # Route legacy paths
    if (req.url ~ "^/legacy/") {
        set req.backend_hint = legacy_service;
        return (pass);
    }

    # Everything else goes to web_production
    set req.backend_hint = web_production;
}

sub vcl_backend_response {
    # Add header to identify which backend was used
    if (bereq.backend == api_production) {
        set beresp.http.X-Backend-Used = "api_production";
    } elsif (bereq.backend == web_production) {
        set beresp.http.X-Backend-Used = "web_production";
    } elsif (bereq.backend == legacy_service) {
        set beresp.http.X-Backend-Used = "legacy_service";
    }
}
