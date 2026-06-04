<!-- markdownlint-disable MD041 -->

Numeric fields often arrive as strings, for example from labels, annotations,
or environment variables. `to_number` converts a string (or boolean) into a
number so it can be compared numerically. Compared as strings, `"80"` would
sort after `"1024"`; converting first makes the comparison correct.
