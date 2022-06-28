package sys

import "context"

// ClockResolution is a positive granularity of clock precision in
// nanoseconds. For example, if the resolution is 1us, this returns 1000.
//
// Note: Some implementations return arbitrary resolution because there's
// no perfect alternative. For example, according to the source in time.go,
// windows monotonic resolution can be 15ms. See /RATIONALE.md.
type ClockResolution uint32

// Walltime returns the current time in epoch seconds with a nanosecond fraction.
type Walltime func(context.Context) (sec int64, nsec int32)

// Nanotime returns nanoseconds since an arbitrary start point, used to measure
// elapsed time. This is sometimes referred to as a tick or monotonic time.
//
// Note: There are no constraints on the value return except that it
// increments. For example, -1 is a valid if the next value is >= 0.
type Nanotime func(context.Context) int64

// Nanosleep puts the current goroutine to sleep for at least ns nanoseconds.
type Nanosleep func(ctx context.Context, ns int64)
