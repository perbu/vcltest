vcl 4.1;

backend default {
    .host = "127.0.0.1";
    .port = "8080";
}

sub vcl_backend_response {
    # Short TTL with long grace period for testing
    # In production, you'd typically use longer values
    set beresp.ttl = 30s;
    set beresp.grace = 300s;  # Serve stale for 5 minutes if backend down
}

sub vcl_deliver {
    # Mark response as stale when serving from grace period
    # obj.ttl < 0s means the TTL has expired but we're within grace
    if (obj.ttl < 0s) {
        set resp.http.X-Varnish-Stale = "1";
    }

    # Debug headers - useful for understanding cache state
    set resp.http.X-Cache-TTL = obj.ttl;
    set resp.http.X-Cache-Grace = obj.grace;
}

sub vcl_backend_error {
    # Custom error page when backend fails and no grace content available
    set beresp.http.Content-Type = "text/html";
    synthetic({"<!DOCTYPE html>
<html>
<head><title>Backend Error</title></head>
<body>
<h1>Service Temporarily Unavailable</h1>
<p>The backend server is not responding.</p>
</body>
</html>"});
    return (deliver);
}
