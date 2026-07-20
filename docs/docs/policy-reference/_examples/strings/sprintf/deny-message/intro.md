<!-- markdownlint-disable MD041 -->

`sprintf` formats a string with values from the policy. Admission and
authorization policies use it so users see *which* field failed and *what*
value was rejected, not only a bare `false`.
