<!-- markdownlint-disable MD041 -->

A naming convention requires every configuration key to be lowercase. This
example reads the supplied config's top-level keys with `object.keys`, then
collects any that differ from their lowercase form so the offending keys are
reported rather than just a pass or fail.
