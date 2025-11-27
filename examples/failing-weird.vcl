vcl 4.1;

backend default {
    .host = "backend.example.com";
    .port = "80";
}

sub vcl_recv {
    set req.http.X-Forwarded-Proto = "https";     # Add X-Forwarded-Proto header to indicate the original protocol
    set req.http.X-Real-IP = "192.168.1.100";    # Add X-Real-IP header (simulating proxy behavior)
    if (req.url ~ "^/api/") {     # Remove cookies for API requests (privacy/caching)
        unset req.http.Cookie;
    } else {
        set req.http.Banana = "Yummy";
    }
    if (req.url ~ "^/v1/") {     # Rewrite URL prefix
        set req.url = regsub(req.url, "^/v1/", "/api/v1/");
    }
}
