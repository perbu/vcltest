vcl 4.1;

backend default {
    .host = "secure.example.com";
    .port = "443";
}

sub vcl_recv {
    # Check for access control header
    if (req.http.X-Letmein == "On") {
        return (pass);
    }

    # Deny access if header is missing or wrong
    return (synth(403, "Forbidden"));
}
