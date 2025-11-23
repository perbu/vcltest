vcl 4.1;

backend default {
    .host = "backend.example.com";
    .port = "80";
}

sub vcl_recv {
    # Handle special paths
    if (req.url == "/health") {
        return (synth(200, "OK"));
    }

    if (req.url == "/redirect") {
        return (synth(301, "Moved"));
    }

    # Pass everything else to backend
    return (pass);
}

sub vcl_backend_response {
    # Add custom header from backend
    set beresp.http.X-Backend = "hit";
    return (deliver);
}

sub vcl_deliver {
    # Add VCL version header
    set resp.http.X-VCL-Version = "4.1";
    return (deliver);
}

sub vcl_synth {
    if (resp.status == 301) {
        set resp.http.Location = "/new-location";
    }
    return (deliver);
}
