<!-- markdownlint-disable MD041 -->

`round` returns the nearest whole number (half away from zero). Gauge and
quota policies use it so fractional utilization becomes a stable integer
percent before comparison or display.
