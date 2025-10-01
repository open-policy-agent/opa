package ll

import (
	"fmt"
	"github.com/olekukonko/ll/lx"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

const (
	maxRecursionDepth = 20      // Maximum depth for recursive type handling to prevent stack overflow
	nilString         = "<nil>" // String representation for nil values
	unexportedString  = "<?>"   // String representation for unexported fields
)

// Concat efficiently concatenates values without a separator using the default logger.
// It converts each argument to a string and joins them directly, optimizing for performance
// in logging scenarios. Thread-safe as it does not modify shared state.
// Example:
//
//	msg := ll.Concat("Hello", 42, true) // Returns "Hello42true"
func Concat(args ...any) string {
	return concat(args...)
}

// ConcatSpaced concatenates values with a space separator using the default logger.
// It converts each argument to a string and joins them with spaces, suitable for log message
// formatting. Thread-safe as it does not modify shared state.
// Example:
//
//	msg := ll.ConcatSpaced("Hello", 42, true) // Returns "Hello 42 true"
func ConcatSpaced(args ...any) string {
	return concatSpaced(args...)
}

// ConcatAll concatenates elements with a separator, prefix, and suffix using the default logger.
// It combines before, main, and after arguments with the specified separator, optimizing memory
// allocation for logging. Thread-safe as it does not modify shared state.
// Example:
//
//	msg := ll.ConcatAll(",", []any{"prefix"}, []any{"suffix"}, "main")
//	// Returns "prefix,main,suffix"
func ConcatAll(sep string, before, after []any, args ...any) string {
	return concatenate(sep, before, after, args...)
}

// concat efficiently concatenates values without a separator.
// It converts each argument to a string and joins them directly, optimizing for performance
// in logging scenarios. Used internally by Concat and other logging functions.
// Example:
//
//	msg := concat("Hello", 42, true) // Returns "Hello42true"
func concat(args ...any) string {
	return concatWith("", args...)
}

// concatSpaced concatenates values with a space separator.
// It converts each argument to a string and joins them with spaces, suitable for formatting
// log messages. Used internally by ConcatSpaced.
// Example:
//
//	msg := concatSpaced("Hello", 42, true) // Returns "Hello 42 true"
func concatSpaced(args ...any) string {
	return concatWith(lx.Space, args...)
}

// concatWith concatenates values with a specified separator using optimized type handling.
// It builds a string from arguments, handling various types efficiently (strings, numbers,
// structs, etc.), and is used by concat and concatSpaced for log message construction.
// Thread-safe as it does not modify shared state.
// Example:
//
//	msg := concatWith(",", "Hello", 42, true) // Returns "Hello,42,true"
func concatWith(sep string, args ...any) string {
	switch len(args) {
	case 0:
		return ""
	case 1:
		return concatToString(args[0])
	}

	var b strings.Builder
	b.Grow(concatEstimateArgs(sep, args))

	for i, arg := range args {
		if i > 0 {
			b.WriteString(sep)
		}
		concatWriteValue(&b, arg, 0)
	}

	return b.String()
}

// concatenate concatenates elements with separators, prefixes, and suffixes efficiently.
// It combines before, main, and after arguments with the specified separator, optimizing
// memory allocation for complex log message formatting. Used internally by ConcatAll.
// Example:
//
//	msg := concatenate(",", []any{"prefix"}, []any{"suffix"}, "main")
//	// Returns "prefix,main,suffix"
func concatenate(sep string, before []any, after []any, args ...any) string {
	totalLen := len(before) + len(after) + len(args)
	switch totalLen {
	case 0:
		return ""
	case 1:
		switch {
		case len(before) > 0:
			return concatToString(before[0])
		case len(args) > 0:
			return concatToString(args[0])
		default:
			return concatToString(after[0])
		}
	}

	var b strings.Builder
	b.Grow(concatEstimateTotal(sep, before, after, args))

	// Write before elements
	concatWriteGroup(&b, sep, before)

	// Write main arguments
	if len(before) > 0 && len(args) > 0 {
		b.WriteString(sep)
	}
	concatWriteGroup(&b, sep, args)

	// Write after elements
	if len(after) > 0 && (len(before) > 0 || len(args) > 0) {
		b.WriteString(sep)
	}
	concatWriteGroup(&b, sep, after)

	return b.String()
}

// concatWriteGroup writes a group of arguments to a strings.Builder with a separator.
// It handles each argument by converting it to a string, used internally by concatenate
// to process before, main, or after groups in log message construction.
// Example:
//
//	var b strings.Builder
//	concatWriteGroup(&b, ",", []any{"a", 42}) // Writes "a,42" to b
func concatWriteGroup(b *strings.Builder, sep string, group []any) {
	for i, arg := range group {
		if i > 0 {
			b.WriteString(sep)
		}
		concatWriteValue(b, arg, 0)
	}
}

// concatToString converts a single argument to a string efficiently.
// It handles common types (string, []byte, fmt.Stringer) with minimal overhead and falls
// back to fmt.Sprint for other types. Used internally by concat and concatenate.
// Example:
//
//	s := concatToString("Hello") // Returns "Hello"
//	s := concatToString([]byte{65, 66}) // Returns "AB"
func concatToString(arg any) string {
	switch v := arg.(type) {
	case string:
		return v
	case []byte:
		return *(*string)(unsafe.Pointer(&v))
	case fmt.Stringer:
		return v.String()
	case error:
		return v.Error()
	default:
		return fmt.Sprint(v)
	}
}

// concatEstimateTotal estimates the total string length for concatenate.
// It calculates the expected size of the concatenated string, including before, main, and
// after arguments with separators, to preallocate the strings.Builder capacity.
// Example:
//
//	size := concatEstimateTotal(",", []any{"prefix"}, []any{"suffix"}, "main")
//	// Returns estimated length for "prefix,main,suffix"
func concatEstimateTotal(sep string, before, after, args []any) int {
	size := 0
	if len(before) > 0 {
		size += concatEstimateArgs(sep, before)
	}
	if len(args) > 0 {
		if size > 0 {
			size += len(sep)
		}
		size += concatEstimateArgs(sep, args)
	}
	if len(after) > 0 {
		if size > 0 {
			size += len(sep)
		}
		size += concatEstimateArgs(sep, after)
	}
	return size
}

// concatEstimateArgs estimates the string length for a group of arguments.
// It sums the estimated sizes of each argument plus separators, used by concatEstimateTotal
// and concatWith to optimize memory allocation for log message construction.
// Example:
//
//	size := concatEstimateArgs(",", []any{"hello", 42}) // Returns estimated length for "hello,42"
func concatEstimateArgs(sep string, args []any) int {
	if len(args) == 0 {
		return 0
	}
	size := len(sep) * (len(args) - 1)
	for _, arg := range args {
		size += concatEstimateSize(arg)
	}
	return size
}

// concatEstimateSize estimates the string length for a single argument.
// It provides size estimates for various types (strings, numbers, booleans, etc.) to
// optimize strings.Builder capacity allocation in logging functions.
// Example:
//
//	size := concatEstimateSize("hello") // Returns 5
//	size := concatEstimateSize(42) // Returns ~2
func concatEstimateSize(arg any) int {
	switch v := arg.(type) {
	case string:
		return len(v)
	case []byte:
		return len(v)
	case int:
		return concatNumLen(int64(v))
	case int64:
		return concatNumLen(v)
	case int32:
		return concatNumLen(int64(v))
	case int16:
		return concatNumLen(int64(v))
	case int8:
		return concatNumLen(int64(v))
	case uint:
		return concatNumLen(uint64(v))
	case uint64:
		return concatNumLen(v)
	case uint32:
		return concatNumLen(uint64(v))
	case uint16:
		return concatNumLen(uint64(v))
	case uint8:
		return concatNumLen(uint64(v))
	case float64:
		return 24 // Max digits for float64
	case float32:
		return 16 // Max digits for float32
	case bool:
		if v {
			return 4 // "true"
		}
		return 5 // "false"
	case fmt.Stringer:
		return 16 // Conservative estimate
	default:
		return 16 // Default estimate
	}
}

// concatNumLen estimates the string length for a signed or unsigned integer.
// It returns a conservative estimate (20 digits) for int64 or uint64 values, including
// a sign for negative numbers, used by concatEstimateSize for memory allocation.
// Example:
//
//	size := concatNumLen(int64(-123)) // Returns 20
//	size := concatNumLen(uint64(123)) // Returns 20
func concatNumLen[T int64 | uint64](v T) int {
	if v < 0 {
		return 20 // Max digits for int64 + sign
	}
	return 20 // Max digits for uint64
}

// concatWriteValue writes a formatted value to a strings.Builder with recursion depth tracking.
// It handles various types (strings, numbers, structs, slices, etc.) and prevents infinite
// recursion by limiting depth. Used internally by concatWith and concatWriteGroup for log
// message formatting.
// Example:
//
//	var b strings.Builder
//	concatWriteValue(&b, "hello", 0) // Writes "hello" to b
//	concatWriteValue(&b, []int{1, 2}, 0) // Writes "[1,2]" to b
func concatWriteValue(b *strings.Builder, arg any, depth int) {
	if depth > maxRecursionDepth {
		b.WriteString("...")
		return
	}

	if arg == nil {
		b.WriteString(nilString)
		return
	}

	if s, ok := arg.(fmt.Stringer); ok {
		b.WriteString(s.String())
		return
	}

	switch v := arg.(type) {
	case string:
		b.WriteString(v)
	case []byte:
		b.Write(v)
	case int:
		b.WriteString(strconv.FormatInt(int64(v), 10))
	case int64:
		b.WriteString(strconv.FormatInt(v, 10))
	case int32:
		b.WriteString(strconv.FormatInt(int64(v), 10))
	case int16:
		b.WriteString(strconv.FormatInt(int64(v), 10))
	case int8:
		b.WriteString(strconv.FormatInt(int64(v), 10))
	case uint:
		b.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint64:
		b.WriteString(strconv.FormatUint(v, 10))
	case uint32:
		b.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint16:
		b.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint8:
		b.WriteString(strconv.FormatUint(uint64(v), 10))
	case float64:
		b.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	case float32:
		b.WriteString(strconv.FormatFloat(float64(v), 'f', -1, 32))
	case bool:
		if v {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	default:
		val := reflect.ValueOf(arg)
		if val.Kind() == reflect.Ptr {
			if val.IsNil() {
				b.WriteString(nilString)
				return
			}
			val = val.Elem()
		}

		switch val.Kind() {
		case reflect.Slice, reflect.Array:
			concatFormatSlice(b, val, depth)
		case reflect.Struct:
			concatFormatStruct(b, val, depth)
		default:
			fmt.Fprint(b, v)
		}
	}
}

// concatFormatSlice formats a slice or array for logging.
// It writes the elements in a bracketed, comma-separated format, handling nested types
// recursively with depth tracking. Used internally by concatWriteValue for log message formatting.
// Example:
//
//	var b strings.Builder
//	val := reflect.ValueOf([]int{1, 2})
//	concatFormatSlice(&b, val, 0) // Writes "[1,2]" to b
func concatFormatSlice(b *strings.Builder, val reflect.Value, depth int) {
	b.WriteByte('[')
	for i := 0; i < val.Len(); i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		concatWriteValue(b, val.Index(i).Interface(), depth+1)
	}
	b.WriteByte(']')
}

// concatFormatStruct formats a struct for logging.
// It writes the struct’s exported fields in a bracketed, name:value format, handling nested
// types recursively with depth tracking. Unexported fields are represented as "<?>".
// Used internally by concatWriteValue for log message formatting.
// Example:
//
//	var b strings.Builder
//	val := reflect.ValueOf(struct{ Name string }{Name: "test"})
//	concatFormatStruct(&b, val, 0) // Writes "[Name:test]" to b
func concatFormatStruct(b *strings.Builder, val reflect.Value, depth int) {
	typ := val.Type()
	b.WriteByte('[')

	first := true
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		if !first {
			b.WriteString("; ")
		}
		first = false

		b.WriteString(field.Name)
		b.WriteByte(':')

		if !fieldValue.CanInterface() {
			b.WriteString(unexportedString)
			continue
		}

		concatWriteValue(b, fieldValue.Interface(), depth+1)
	}

	b.WriteByte(']')
}
