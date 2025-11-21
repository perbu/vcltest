vcl 4.1;

backend default {
    .host = "127.0.0.1";
    .port = "8080";
}

sub vcl_recv {
    # Check for access control header
    if (req.http.X-Letmein == "On") {
        return (pass);
    }

    # Deny access if header is missing or wrong
    return (synth(403, "Forbidden"));
}

sub vcl_backend_response {
    return (deliver);
}
