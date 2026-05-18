<!-- markdownlint-disable MD041 -->

`array.slice` returns the sub-array from index `start` (inclusive) to `stop`
(exclusive). A common use is extracting a fixed-length prefix from a
variable-length path after splitting it into segments, so the routing logic
stays independent of how deep the path goes.
