# This file is included by test_with_include.vcl

sub custom_admin_check {
    if (req.url ~ "^/admin") {
        return (synth(403, "Admin blocked from include"));
    }
}
