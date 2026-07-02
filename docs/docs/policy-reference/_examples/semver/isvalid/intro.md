<!-- markdownlint-disable MD041 -->

The `result := semver.is_valid(vsn)` function checks to see if a version
string is of the form: `MAJOR.MINOR.PATCH[-PRERELEASE][+METADATA]`, where
items in square braces are optional elements.

:::warning
When working with Go-style semantic versions, remember to remove the
leading `v` character, or the semver string will be marked as invalid!
:::
