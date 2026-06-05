<!-- markdownlint-disable MD041 -->

A container image reference packs the registry, repository, and tag into one
string. This example pulls it apart in two steps: first splitting the repository
from the tag on the `:` separator, then breaking the repository into its
`/` separated path segments so each part can be checked on its own.
