vcl 4.1;

include "included.vcl";

backend default {
    .host = "127.0.0.1";
    .port = "8080";
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
