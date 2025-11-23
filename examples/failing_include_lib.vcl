sub check_admin_headers {
    # Include VCL: Add tracking header
    set req.http.X-Admin-Request = "true";

    # Include VCL: Check for admin token
    if (req.http.X-Admin-Token) {
        set req.http.X-Admin-Authorized = "yes";
    } else {
        set req.http.X-Admin-Authorized = "no";
    }
    # Include VCL: Don't return - let built-in VCL handle caching decision
}
