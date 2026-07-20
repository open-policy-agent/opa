<!-- markdownlint-disable MD041 -->

Container specs often express memory with different suffixes (`Mi`, `Gi`,
bare numbers). `units.parse` turns those strings into numbers so a policy
can compare them. Here, a container is rejected when its memory limit is
higher than the namespace cap.
