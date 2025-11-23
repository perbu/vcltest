vcl 4.1;

backend default {
    .host = "backend.example.com";
    .port = "80";
}

sub vcl_recv {
    return (hash);
}

sub vcl_backend_response {
    # Respect backend Cache-Control
    return (deliver);
}

sub vcl_deliver {
    set resp.http.X-VCL-Test = "backend-reconfig-demo";
}
