<!-- markdownlint-disable MD041 -->

`time.format` can be used to provide information to the
user in a human-readable format in error messages. Error codes
and local times can be useful when debugging or troubleshooting
and so in many cases returning them from policy decisions can
be helpful.

In this example we see a user is not an admin and is denied access,
the policy response is a message that includes the current time
and an error code to help them debug.
