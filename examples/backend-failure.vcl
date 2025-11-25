vcl 4.1;

backend default {
    .host = "127.0.0.1";
    .port = "8080";
}

sub vcl_recv {
    return (pass);
}

sub vcl_backend_error {
    # This subroutine is called when Varnish cannot connect to the backend
    # or the backend connection is reset (failure_mode: failed)
    set beresp.http.Content-Type = "text/plain";
    set beresp.status = 503;
    synthetic("Backend unavailable");
    return (deliver);
}

sub vcl_deliver {
    return (deliver);
}
