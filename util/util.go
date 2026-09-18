// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package util provides generic utilities used throughout the policy engine.
//
// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package util

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	v1 "github.com/open-policy-agent/opa/v1/util"
)

// DefaultBackoff returns a delay with an exponential backoff based on the
// number of retries.
func DefaultBackoff(base, maxNS float64, retries int) time.Duration {
	return v1.DefaultBackoff(base, maxNS, retries)
}

// Backoff returns a delay with an exponential backoff based on the number of
// retries. Same algorithm used in gRPC.
func Backoff(base, maxNS, jitter, factor float64, retries int) time.Duration {
	return v1.Backoff(base, maxNS, jitter, factor, retries)
}

// Close reads the remaining bytes from the response and then closes it to
// ensure that the connection is freed. If the body is not read and closed, a
// leak can occur.
func Close(resp *http.Response) {
	v1.Close(resp)
}

// Compare returns 0 if a equals b, -1 if a is less than b, and 1 if b is than a.
//
// For comparison between values of different types, the following ordering is used:
// nil < bool < int, float64 < string < []any < map[string]any. Slices and maps
// are compared recursively. If one slice or map is a subset of the other slice or map
// it is considered "less than". Nil is always equal to nil.
func Compare(a, b any) int {
	return v1.Compare(a, b)
}

// EnumFlag implements the pflag.Value interface to provide enumerated command
// line parameter values.
type EnumFlag = v1.EnumFlag

// NewEnumFlag returns a new EnumFlag that has a defaultValue and vs enumerated
// values.
func NewEnumFlag(defaultValue string, vs []string) *EnumFlag {
	return v1.NewEnumFlag(defaultValue, vs)
}

// Traversal defines a basic interface to perform traversals.
type Traversal = v1.Traversal

// Equals should return true if node "u" equals node "v".
type Equals = v1.Equals

// Iter should return true to indicate stop.
type Iter = v1.Iter

// DFS performs a depth first traversal calling f for each node starting from u.
// If f returns true, traversal stops and DFS returns true.
func DFS(t Traversal, f Iter, u T) bool {
	return v1.DFS(t, f, u)
}

// BFS performs a breadth first traversal calling f for each node starting from
// u. If f returns true, traversal stops and BFS returns true.
func BFS(t Traversal, f Iter, u T) bool {
	return v1.BFS(t, f, u)
}

// DFSPath returns a path from node a to node z found by performing
// a depth first traversal. If no path is found, an empty slice is returned.
func DFSPath(t Traversal, eq Equals, a, z T) []T {
	return v1.DFSPath(t, eq, a, z)
}

// T is a concise way to refer to T.
type T = v1.T

// HashMap represents a key/value map.
type HashMap = v1.HashMap

// NewHashMap returns a new empty HashMap.
func NewHashMap(eq func(T, T) bool, hash func(T) int) *HashMap {
	return v1.NewHashMap(eq, hash)
}

// UnmarshalJSON parses the JSON encoded data and stores the result in the value
// pointed to by x.
//
// This function is intended to be used in place of the standard json.Marshal
// function when json.Number is required.
func UnmarshalJSON(bs []byte, x any) error {
	return v1.UnmarshalJSON(bs, x)
}

// NewJSONDecoder returns a new decoder that reads from r.
//
// This function is intended to be used in place of the standard json.NewDecoder
// when json.Number is required.
func NewJSONDecoder(r io.Reader) *json.Decoder {
	return v1.NewJSONDecoder(r)
}

// MustUnmarshalJSON parse the JSON encoded data and returns the result.
//
// If the data cannot be decoded, this function will panic. This function is for
// test purposes.
func MustUnmarshalJSON(bs []byte) any {
	return v1.MustUnmarshalJSON(bs)
}

// MustMarshalJSON returns the JSON encoding of x
//
// If the data cannot be encoded, this function will panic. This function is for
// test purposes.
func MustMarshalJSON(x any) []byte {
	return v1.MustMarshalJSON(x)
}

// RoundTrip encodes to JSON, and decodes the result again.
//
// Thereby, it is converting its argument to the representation expected by
// rego.Input and inmem's Write operations. Works with both references and
// values.
func RoundTrip(x *any) error {
	return v1.RoundTrip(x)
}

// Reference returns a pointer to its argument unless the argument already is
// a pointer. If the argument is **t, or ***t, etc, it will return *t.
//
// Used for preparing Go types (including pointers to structs) into values to be
// put through util.RoundTrip().
func Reference(x any) *any {
	return v1.Reference(x)
}

// Unmarshal decodes a YAML, JSON or JSON extension value into the specified type.
func Unmarshal(bs []byte, v any) error {
	return v1.Unmarshal(bs, v)
}

// Values returns a slice of values from any map. Copied from golang.org/x/exp/maps.
func Values[M ~map[K]V, K comparable, V any](m M) []V {
	return v1.Values(m)
}

// LIFO represents a simple LIFO queue.
type LIFO = v1.LIFO

// NewLIFO returns a new LIFO queue containing elements ts starting with the
// left-most argument at the bottom.
func NewLIFO(ts ...T) *LIFO {
	return v1.NewLIFO(ts...)
}

// FIFO represents a simple FIFO queue.
type FIFO = v1.FIFO

// NewFIFO returns a new FIFO queue containing elements ts starting with the
// left-most argument at the front.
func NewFIFO(ts ...T) *FIFO {
	return v1.NewFIFO(ts...)
}

// Note(philipc): Originally taken from server/server.go
// The DecodingLimitHandler handles validating that the gzip payload is within the
// allowed max size limit. Thus, in the event of a forged payload size trailer,
// the worst that can happen is that we waste memory up to the allowed max gzip
// payload size, but not an unbounded amount of memory, as was potentially
// possible before.
func ReadMaybeCompressedBody(r *http.Request) ([]byte, error) {
	return v1.ReadMaybeCompressedBody(r)
}

// TimerWithCancel exists because of memory leaks when using
// time.After in select statements. Instead, we now manually create timers,
// wait on them, and manually free them.
//
// See this for more details:
// https://www.arangodb.com/2020/09/a-story-of-a-memory-leak-in-go-how-to-properly-use-time-after/
//
// Note: This issue is fixed in Go 1.23, but this fix helps us until then.
//
// Warning: the cancel cannot be done concurrent to reading, everything should
// work in the same goroutine.
//
// Example:
//
//	for retries := 0; true; retries++ {
//
//		...main logic...
//
//		timer, cancel := utils.TimerWithCancel(utils.Backoff(retries))
//		select {
//		case <-ctx.Done():
//			cancel()
//			return ctx.Err()
//		case <-timer.C:
//			continue
//		}
//	}
func TimerWithCancel(delay time.Duration) (*time.Timer, func()) {
	return v1.TimerWithCancel(delay)
}

// WaitFunc will call passed function at an interval and return nil
// as soon this function returns true.
// If timeout is reached before the passed in function returns true
// an error is returned.
func WaitFunc(fun func() bool, interval, timeout time.Duration) error {
	return v1.WaitFunc(fun, interval, timeout)
}
