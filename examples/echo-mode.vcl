vcl 4.1;

backend default {
    .host = "backend.example.com";
    .port = "80";
}

sub vcl_recv {
    # Add X-Forwarded-Proto header to indicate the original protocol
    set req.http.X-Forwarded-Proto = "https";

    # Add X-Real-IP header (simulating proxy behavior)
    set req.http.X-Real-IP = "192.168.1.100";

    # Remove cookies for API requests (privacy/caching)
    if (req.url ~ "^/api/") {
        unset req.http.Cookie;
    }

    # Rewrite URL prefix
    if (req.url ~ "^/v1/") {
        set req.url = regsub(req.url, "^/v1/", "/api/v1/");
    }

    return (pass);
}

sub vcl_deliver {
    return (deliver);
}
