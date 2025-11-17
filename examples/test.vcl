vcl 4.1;

backend default {
    .host = "127.0.0.1";
    .port = "8080";
}

sub vcl_recv {
    # Block admin access
    if (req.url ~ "^/admin") {
        return (synth(403, "Forbidden"));
    }

    # API requests bypass cache
    if (req.url ~ "^/api/") {
        return (pass);
    }

    # Static assets get cached
    if (req.url ~ "\.(jpg|png|css|js)$") {
        unset req.http.Cookie;
        return (hash);
    }

    # Everything else
    return (hash);
}

sub vcl_backend_response {
    # Cache static assets for 1 hour
    if (bereq.url ~ "\.(jpg|png|css|js)$") {
        set beresp.ttl = 1h;
    }
}
