<!-- markdownlint-disable MD041 -->

Managing access control in web applications is crucial for security. The
following example uses Rego's `regex.match` to define role-based access to
different URL paths. By associating URL patterns with user roles like "intern"
and "admin," it ensures that users only access authorized paths.
