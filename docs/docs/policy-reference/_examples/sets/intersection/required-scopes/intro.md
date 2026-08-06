<!-- markdownlint-disable MD041 -->

`intersection` returns the values common to every set you pass in. A typical
use is comparing a caller's granted scopes with the scopes an endpoint
requires: if the intersection is smaller than the requirement, something is
missing.
