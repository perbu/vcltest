vcl 4.1;

backend default {
    .host = "api.example.com";
    .port = "443";
}

sub vcl_recv {
    # Block admin paths
    if (req.url ~ "^/admin") {
        return (synth(403, "Forbidden"));
    }

    # Allow API paths
    if (req.url ~ "^/api/") {
        return (pass);
    }

    # Everything else goes to cache
    return (hash);
}

sub vcl_backend_response {
    set beresp.http.X-Backend-Hit = "true";
    return (deliver);
}

sub vcl_deliver {
    set resp.http.X-Served-By = "varnish";
    return (deliver);
}
