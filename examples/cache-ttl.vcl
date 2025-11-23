vcl 4.1;

backend default {
    .host = "cache-backend.example.com";
    .port = "80";
}

sub vcl_recv {
    # Allow caching
    return (hash);
}

sub vcl_backend_response {
    # Respect backend Cache-Control header
    # Default TTL is set by Varnish if not specified
    return (deliver);
}

sub vcl_deliver {
    # Add custom header to indicate VCL version
    set resp.http.X-VCL-Test = "cache-ttl";
}
