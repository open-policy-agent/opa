<!-- markdownlint-disable MD041 -->

A deployment pipeline defines ordered steps to bring a service up. When tearing
the service down, those steps must run in reverse. This example reverses the
"up" sequence and checks that a proposed teardown plan follows the correct order.
