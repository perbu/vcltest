vcl 4.1;

import std;

backend default {
    .host = "time-backend.example.com";
    .port = "80";
}

sub vcl_recv {
    # Log the current time as seen by VCL
    # 'now' is a built-in TIME variable representing current time (epoch seconds)
    std.log("VCL_TIME:" + now);

    return (synth(200, "OK"));
}

sub vcl_synth {
    # Include time in response for easy verification
    set resp.http.X-VCL-Time = now;
    set resp.http.X-VCL-Time-Readable = std.strftime(now, "%Y-%m-%d %H:%M:%S");
    set resp.body = "Current VCL time (epoch): " + now;
    return (deliver);
}
