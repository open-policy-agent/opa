<!-- markdownlint-disable MD041 -->

When a policy builds a user-facing error, it can accidentally include a
sensitive header value. `replace` redacts that substring before the message
goes back to the caller.
