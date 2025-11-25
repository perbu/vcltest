vcl 4.1;

backend user_api {
    .host = "users.production.example.com";
    .port = "443";
}

backend product_api {
    .host = "products.production.example.com";
    .port = "443";
}

backend analytics_api {
    .host = "analytics.production.example.com";
    .port = "443";
}

sub vcl_recv {
    # Only allow POST requests to these endpoints
    if (req.method == "POST") {
        # Route user-related POST requests
        if (req.url ~ "^/api/users") {
            set req.backend_hint = user_api;
            return (pass);  # Don't cache POST requests
        }

        # Route product-related POST requests
        if (req.url ~ "^/api/products") {
            set req.backend_hint = product_api;
            return (pass);
        }

        # Route analytics events
        if (req.url ~ "^/api/events") {
            set req.backend_hint = analytics_api;
            return (pass);
        }
    }

    # Reject other methods/paths
    return (synth(405, "Method Not Allowed"));
}

sub vcl_backend_response {
    # Never cache POST responses
    if (bereq.method == "POST") {
        set beresp.uncacheable = true;
        set beresp.ttl = 0s;
    }
}
