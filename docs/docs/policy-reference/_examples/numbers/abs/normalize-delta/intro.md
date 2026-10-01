<!-- markdownlint-disable MD041 -->

`abs` returns the absolute value of a number. Policies can use it when
evaluating timestamp drift or clock skew between client and server, ensuring that
a value does not deviate in either direction beyond an acceptable threshold.
