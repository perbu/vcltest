vcl 4.1;

backend api_backend {
    .host = "__BACKEND_HOST_API_BACKEND__";
    .port = "__BACKEND_PORT_API_BACKEND__";
}

backend web_backend {
    .host = "__BACKEND_HOST_WEB_BACKEND__";
    .port = "__BACKEND_PORT_WEB_BACKEND__";
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
