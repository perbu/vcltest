vcl 4.1;

backend api_backend {
    .host = "api.production.example.com";
    .port = "443";
}

backend web_backend {
    .host = "web.production.example.com";
    .port = "443";
}

sub vcl_recv {
    # Route API requests to api_backend
    if (req.url ~ "^/api/") {
        set req.backend_hint = api_backend;
    } else {
        # Everything else goes to web_backend
        set req.backend_hint = web_backend;
    }
}
