The goal of this project is to help users verify that the VCL they've made is working.

The application will do the following:

1. Start a varnishd instance.

Then for each test specification we do the following:

1. Start varnishlog-json
2. Create one HTTP backend, according to test specificantion.
3. Alter the VCL file so it is "traceable"
4. Connect to the varnishd instance and load the above VCL.
5. Execute the requests from the test specification.

This raises quite a few questions.

# Trace VCL

It is crucial that when we are executing the tests, we need to understand what is happening in the
VCL runtime. The way we should do this is by having a log statement for every line of logic. This way
we can see trace the log execution of the VCL code and follow the flow.

In addition, we will notice what headers are being set and what values they have.

So, when we trigger a client request, varnishlog will capture the log entries.

# Client Request

Initially I think we should make the client requests simple. We can just specify a METHOD/URL, headers and what the
expected response should be.

# Client Response

We need to check that the response is what we expected. Perhaps we should just take inspiration from
a well known test library.

Please suggest what kind of expect statements we should support.

# HTTP Backend

We should just spin up a simple HTTP server.

# Log capture

When starting a test, we start by executing varnishlog-json and capture the output.

# Later expansion

- Multiple HTTP backends
- Multiple Client Requests
- Programmable client requests
- Programmable backend responses

# Instructions for code style

Simplicity over flexibility.
Tests should only use the stdlib.