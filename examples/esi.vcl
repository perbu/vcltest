vcl 4.1;

backend default {
    .host = "backend.example.com";
    .port = "80";
}

sub vcl_recv {
    return (pass);
}

sub vcl_backend_response {
    # Enable ESI processing on responses from backend
    set beresp.do_esi = true;
}
