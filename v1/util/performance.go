package util

import (
	"bytes"
	"cmp"
	"encoding"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unsafe"
)

// SyncPool is a generic sync.Pool for type T, providing some convenience
// over sync.Pool directly: [SyncPool.Put] ensures that nil values are not
// put into the pool, and [SyncPool.Get] returns a pointer to T without having
// to do a type assertion at the call site.
type SyncPool[T any] struct {
	Pool sync.Pool
}

func NewSyncPool[T any]() *SyncPool[T] {
	return &SyncPool[T]{
		Pool: sync.Pool{
			New: func() any {
				return new(T)
			},
		},
	}
}

func (p *SyncPool[T]) Get() *T {
	return p.Pool.Get().(*T)
}

func (p *SyncPool[T]) Put(x *T) {
	if x != nil {
		p.Pool.Put(x)
	}
}

// resettable is implemented by *T when used with [ResettablePool], allowing
// pooled values to clear their internal state (e.g. drop pointers so they
// don't outlive their useful life) before being returned to the pool.
type resettable[T any] interface {
	*T
	Reset()
}

// ResettablePool is like [SyncPool], but for types whose pointer clears its
// own fields via a Reset method before being pooled. Unlike a runtime
// interface check on every Put, the PT type parameter is resolved at compile
// time, so there's no extra dispatch cost over a hand-written pool.
type ResettablePool[T any, PT resettable[T]] struct {
	pool sync.Pool
}

func NewResettablePool[T any, PT resettable[T]]() *ResettablePool[T, PT] {
	return &ResettablePool[T, PT]{
		pool: sync.Pool{
			New: func() any {
				return new(T)
			},
		},
	}
}

func (p *ResettablePool[T, PT]) Get() *T {
	return p.pool.Get().(*T)
}

func (p *ResettablePool[T, PT]) Put(x *T) {
	if x != nil {
		PT(x).Reset()
		p.pool.Put(x)
	}
}

// NewPtrSlice returns a slice of pointers to T with length n,
// with only 2 allocations performed no matter the size of n.
// See:
// https://gist.github.com/CAFxX/e96e8a5c3841d152f16d266a1fe7f8bd#slices-of-pointers
func NewPtrSlice[T any](n int) []*T {
	return GrowPtrSlice[T](nil, n)
}

// GrowPtrSlice appends n elements to the slice, each pointing to
// a newly-allocated T. The resulting slice has length equal to len(s)+n.
//
// It performs at most 2 allocations, regardless of n.
func GrowPtrSlice[T any](s []*T, n int) []*T {
	s = slices.Grow(s, n)
	p := make([]T, n)
	for i := range n {
		s = append(s, &p[i])
	}
	return s
}

// Allocation free conversion from []byte to string (unsafe)
// Note that the byte slice must not be modified after conversion
func ByteSliceToString(bs []byte) string {
	return unsafe.String(unsafe.SliceData(bs), len(bs))
}

// Allocation free conversion from ~string to []byte (unsafe)
// Note that the byte slice must not be modified after conversion, and that it
// aliases the string's memory: it is a view of s, not a copy like []byte(s).
func StringToByteSlice[T ~string](s T) []byte {
	if len(s) == 0 {
		// unsafe.StringData's return value is unspecified for the empty string,
		// so don't build a slice on top of it. Doing so currently yields a nil
		// slice, which callers may treat differently from an empty one.
		return []byte{}
	}
	return unsafe.Slice(unsafe.StringData(string(s)), len(s))
}

// NumDigitsInt returns the number of digits in n.
// This is useful for pre-allocating buffers for string conversion.
func NumDigitsInt(n int) int {
	return NumDigitsInt64(int64(n))
}

// NumDigitsInt64 returns the number of digits in n.
// This is useful for pre-allocating buffers for string conversion.
func NumDigitsInt64(n int64) int {
	if n == 0 {
		return 1
	}

	if n < 0 {
		n = -n
	}

	count := 0
	for n > 0 {
		n /= 10
		count++
	}
	return count
}

// NumDigitsUint returns the number of digits in n.
// This is useful for pre-allocating buffers for string conversion.
func NumDigitsUint(n uint64) int {
	if n == 0 {
		return 1
	}

	count := 0
	for n > 0 {
		n /= 10
		count++
	}
	return count
}

// AppendInt is a less messy version of strconv.AppendInt for base 10 ints.
func AppendInt[T Integer](buf []byte, n T) []byte {
	return strconv.AppendInt(buf, int64(n), 10)
}

// WriteInt writes the string form of n to out.
func WriteInt[T Integer](out io.Writer, n T) (int, error) {
	var buf []byte
	if b, ok := out.(*bytes.Buffer); ok {
		buf = b.AvailableBuffer()
	}
	return out.Write(AppendInt(buf, n))
}

// WriteAppender writes the appended text of appender to out.
func WriteAppender[T encoding.TextAppender](out io.Writer, appender T) (int, error) {
	var buf []byte
	if b, ok := out.(*bytes.Buffer); ok {
		buf = b.AvailableBuffer()
	}
	b, err := appender.AppendText(buf)
	if err != nil {
		return 0, err
	}
	return out.Write(b)
}

// Atoi is a convenience function for [Atoi64] where an int is preferable to an int64.
// See the documentation of [Atoi64] for details on the performance benefits of this
// function over strconv.Atoi.
func Atoi(s string) (int, bool) {
	if i, ok := Atoi64(s); ok {
		return int(i), true
	}
	return 0, false
}

// Atoi64 is an alternative implementation of strconv.Atoi which is slightly faster for the
// (for our use case) common case of a successful conversion, and crucially — *much* faster
// for the failure case, as this function allocates nothing for any given input string, while
// strconv.Atoi performs 1-2 allocations on failure in its error handling. The callers in this
// codebase — most notably ast.Number's Int() and Int64() methods — have no interest in the
// details of the failure, and keeping this allocation free means both methods can be used
// not only for conversion, but as a most efficient "IsInt64" check.
// Additionally this function accepts trailing decimal zeroes ("10.00", not "10.01") as that
// makes sense in the context of us using JSON numbers.
func Atoi64(s string) (int64, bool) {
	sLen := len(s)
	if sLen > 0 {
		negative := s[0] == '-'
		if negative || s[0] == '+' {
			s = s[1:]
			sLen--
		}
		if sLen == 0 || sLen > 19 {
			return 0, false
		}

		var pastDecimal bool
		var n int64
		for _, ch := range []byte(s) {
			if ch == '.' {
				if !pastDecimal {
					pastDecimal = true
					continue
				}
				return 0, false
			}
			ch -= '0'
			if pastDecimal {
				if ch == 0 {
					continue
				}
				return 0, false
			}
			if ch > 9 {
				return 0, false
			}
			n = n*10 + int64(ch)
		}
		if !negative && n < 0 {
			return 0, false // overflow
		}
		if negative {
			n = -n
			if n > 0 {
				return 0, false // underflow
			}
		}
		return n, true
	}

	return 0, false
}

// SplitMap calls fn for each delim-separated part of text and returns a slice of the results.
// Cheaper than calling fn on strings.Split(text, delim), as it avoids allocating an intermediate slice of strings.
func SplitMap[T any](text, delim string, fn func(string) T) []T {
	sl := make([]T, 0, strings.Count(text, delim)+1)
	for s := range strings.SplitSeq(text, delim) {
		sl = append(sl, fn(s))
	}
	return sl
}

// SlicePool is a pool for (pointers to) slices of type T.
// It uses sync.Pool to pool the slices, and grows them as needed.
type SlicePool[T any] struct {
	pool sync.Pool
}

// NewSlicePool creates a new SlicePool for slices of type T with the given initial length.
// This number is only a hint, as the slices will grow as needed. For best performance, store
// slices of similar lengths in the same pool.
func NewSlicePool[T any](length int) *SlicePool[T] {
	return &SlicePool[T]{
		pool: sync.Pool{
			New: func() any {
				s := make([]T, length)
				return &s
			},
		},
	}
}

// Get returns a pointer to a slice of type T with the given length
// from the pool. The slice capacity will grow as needed to accommodate
// the requested length. The returned slice will have all its elements
// set to the zero value of T. Returns a pointer to avoid allocating.
func (sp *SlicePool[T]) Get(length int) *[]T {
	s := sp.pool.Get().(*[]T)
	d := *s

	if cap(d) < length {
		d = slices.Grow(d, length)
	}

	d = d[:length] // reslice to requested length, while keeping capacity

	clear(d)

	*s = d
	return s
}

// Put returns a pointer to a slice of type T to the pool.
func (sp *SlicePool[T]) Put(s *[]T) {
	if s != nil {
		sp.pool.Put(s)
	}
}

// SortedFunc is simply a shorthand for [slices.SortFunc] which also returns the sorted slice.
func SortedFunc[T any, S ~[]T](s S, cmp func(a, b T) int) S {
	slices.SortFunc(s, cmp)
	return s
}

// SortedStableFunc is simply a shorthand for [slices.SortStableFunc] which also returns the sorted slice.
func SortedStableFunc[T any, S ~[]T](s S, cmp func(a, b T) int) S {
	slices.SortStableFunc(s, cmp)
	return s
}

// Sorted is simply a shorthand for [slices.Sort] which also returns the sorted slice.
func Sorted[T cmp.Ordered, S ~[]T](s S) S {
	slices.Sort(s)
	return s
}
