vcl 4.1;

include "included.vcl";

backend default {
    .host = "include-test.example.com";
    .port = "80";
}

sub vcl_recv {
    # Call the included subroutine
    call custom_admin_check;

    # API requests bypass cache
    if (req.url ~ "^/api/") {
        return (pass);
    }

    # Everything else
    return (hash);
}
