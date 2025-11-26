vcl 4.1;

backend default {
    .host = "localhost";
    .port = "8080";
}

sub vcl_recv {
    # Check if client has a shard cookie with value "backend-a"
    if (req.http.Cookie ~ "shard=backend-a") {
        set req.http.X-Has-Shard = "yes";
    }
}

sub vcl_deliver {
    # Pass through whether we saw the shard cookie
    if (req.http.X-Has-Shard) {
        set resp.http.X-Shard-Received = "true";
    }
}
