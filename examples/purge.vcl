vcl 4.1;

backend default {
    .host = "content.example.com";
    .port = "80";
}

acl purge_allowed {
    "localhost";
    "127.0.0.1";
}

sub vcl_recv {
    if (req.method == "PURGE") {
        if (!client.ip ~ purge_allowed) {
            return (synth(405, "Not allowed"));
        }
        return (purge);
    }
    return (hash);
}

sub vcl_purge {
    return (synth(200, "Purged"));
}

sub vcl_backend_response {
    # Cache for 1 hour
    set beresp.ttl = 1h;
    return (deliver);
}

sub vcl_deliver {
    # Add header to indicate cache status
    if (obj.hits > 0) {
        set resp.http.X-Cache = "HIT";
    } else {
        set resp.http.X-Cache = "MISS";
    }
}
