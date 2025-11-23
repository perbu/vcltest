vcl 4.1;

include "failing_include_lib.vcl";

backend default {
    .host = "demo.example.com";
    .port = "80";
}

sub vcl_recv {
    # Main VCL: Check if this is an admin request
    if (req.url ~ "^/admin") {
        # Call the included subroutine to handle admin logic
        call check_admin_headers;
    }

    # Main VCL: Check for cache bypass
    if (req.url ~ "^/nocache") {
        return (pass);
    }

    # Let built-in VCL handle everything else (will call vcl_hash, etc.)
}
