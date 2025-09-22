# go-clone: Clone any Go data structure deeply and thoroughly

[![Go](https://github.com/huandu/go-clone/workflows/Go/badge.svg)](https://github.com/huandu/go-clone/actions)
[![Go Doc](https://godoc.org/github.com/huandu/go-clone?status.svg)](https://pkg.go.dev/github.com/huandu/go-clone)
[![Go Report](https://goreportcard.com/badge/github.com/huandu/go-clone)](https://goreportcard.com/report/github.com/huandu/go-clone)
[![Coverage Status](https://coveralls.io/repos/github/huandu/go-clone/badge.svg?branch=master)](https://coveralls.io/github/huandu/go-clone?branch=master)

Package `clone` provides functions to deep clone any Go data. It also provides a wrapper to protect a pointer from any unexpected mutation.

For users who use Go 1.18+, it's recommended to import `github.com/huandu/go-clone/generic` for generic APIs and arena support.

`Clone`/`Slowly` can clone unexported fields and "no-copy" structs as well. Use this feature wisely.

## Install

Use `go get` to install this package.

```shell
go get github.com/huandu/go-clone
```

## Usage

### `Clone` and `Slowly`

If we want to clone any Go value, use `Clone`.

```go
t := &T{...}
v := clone.Clone(t).(*T)
reflect.DeepEqual(t, v) // true
```

For the sake of performance, `Clone` doesn't deal with values containing pointer cycles.
If we need to clone such values, use `Slowly` instead.

```go
type ListNode struct {
    Data int
    Next *ListNode
}
node1 := &ListNode{
    Data: 1,
}
node2 := &ListNode{
    Data: 2,
}
node3 := &ListNode{
    Data: 3,
}
node1.Next = node2
node2.Next = node3
node3.Next = node1

// We must use `Slowly` to clone a circular linked list.
node := Slowly(node1).(*ListNode)

for i := 0; i < 10; i++ {
    fmt.Println(node.Data)
    node = node.Next
}
```

### Generic APIs

Starting from go1.18, Go started to support generic. With generic syntax, `Clone`/`Slowly` and other APIs can be called much cleaner like following.

```go
import "github.com/huandu/go-clone/generic"

type MyType struct {
    Foo string
}

original := &MyType{
    Foo: "bar",
}

// The type of cloned is *MyType instead of interface{}.
cloned := Clone(original)
println(cloned.Foo) // Output: bar
```

It's required to update minimal Go version to 1.18 to opt-in generic syntax. It may not be a wise choice to update this package's `go.mod` and drop so many old Go compilers for such syntax candy. Therefore, I decide to create a new standalone package `github.com/huandu/go-clone/generic` to provide APIs with generic syntax.

For new users who use Go 1.18+, the generic package is preferred and recommended.

### Arena support

Starting from Go1.20, arena is introduced as a new way to allocate memory. It's quite useful to improve overall performance in special scenarios.
In order to clone a value with memory allocated from an arena, there are new methods `ArenaClone` and `ArenaCloneSlowly` available in `github.com/huandu/go-clone/generic`.

```go
// ArenaClone recursively deep clones v to a new value in arena a.
// It works in the same way as Clone, except it allocates all memory from arena.
func ArenaClone[T any](a *arena.Arena, v T) (nv T) 

// ArenaCloneSlowly recursively deep clones v to a new value in arena a.
// It works in the same way as Slowly, except it allocates all memory from arena.
func ArenaCloneSlowly[T any](a *arena.Arena, v T) (nv T)
```

Due to limitations in arena API, memory of the internal data structure of `map` and `chan` is always allocated in heap by Go runtime ([see this issue](https://github.com/golang/go/issues/56230)).

**Warning**: Per [discussion in the arena proposal](https://github.com/golang/go/issues/51317), the arena package may be changed incompatibly or removed in future. All arena related APIs in this package will be changed accordingly.

### Struct tags

There are some struct tags to control how to clone a struct field.

```go
type T struct {
    Normal *int
    Foo    *int `clone:"skip"`       // Skip cloning this field so that Foo will be zero in cloned value.
    Bar    *int `clone:"-"`          // "-" is an alias of skip.
    Baz    *int `clone:"shadowcopy"` // Copy this field by shadow copy.
}

a := 1
t := &T{
    Normal: &a,
    Foo:    &a,
    Bar:    &a,
    Baz:    &a,
}
v := clone.Clone(t).(*T)

fmt.Println(v.Normal == t.Normal) // false
fmt.Println(v.Foo == nil)         // true
fmt.Println(v.Bar == nil)         // true
fmt.Println(v.Baz == t.Baz)       // true
```

### Memory allocations and the `Allocator`

The `Allocator` is designed to allocate memory when cloning. It's also used to hold all customizations, e.g. custom clone functions, scalar types and opaque pointers, etc. There is a default allocator which allocates memory from heap. Almost all public APIs in this package use this default allocator to do their job.

We can control how to allocate memory by creating a new `Allocator` by `NewAllocator`. It enables us to take full control over memory allocation when cloning. See [Allocator sample code](https://pkg.go.dev/github.com/huandu/go-clone#example-Allocator) to understand how to customize an allocator.

Let's take a closer look at the `NewAllocator` function.

```go
func NewAllocator(pool unsafe.Pointer, methods *AllocatorMethods) *Allocator
```

- The first parameter `pool` is a pointer to a memory pool. It's used to allocate memory for cloning. It can be `nil` if we don't need a memory pool.
- The second parameter `methods` is a pointer to a struct which contains all methods to allocate memory. It can be `nil` if we don't need to customize memory allocation.
- The `Allocator` struct is allocated from the `methods.New` or the `methods.Parent` allocator or from heap.

The `Parent` in `AllocatorMethods` is used to indicate the parent of the new allocator. With this feature, we can orgnize allocators into a tree structure. All customizations, including custom clone functions, scalar types and opaque pointers, etc, are inherited from parent allocators.

There are some APIs designed for convenience.

- We can create dedicated allocators for heap or arena by calling `FromHeap()` or `FromArena(a *arena.Arena)`.
- We can call `MakeCloner(allocator)` to create a helper struct with `Clone` and `CloneSlowly` methods in which the type of in and out parameters is `interface{}`.

### Mark struct type as scalar

Some struct types can be considered as scalar.

A well-known case is `time.Time`.
Although there is a pointer `loc *time.Location` inside `time.Time`, we always use `time.Time` by value in all methods.
When cloning `time.Time`, it should be OK to return a shadow copy.

Currently, following types are marked as scalar by default.

- `time.Time`
- `reflect.Value`

If there is any type defined in built-in package should be considered as scalar, please open new issue to let me know.
I will update the default.

If there is any custom type should be considered as scalar, call `MarkAsScalar` to mark it manually. See [MarkAsScalar sample code](https://pkg.go.dev/github.com/huandu/go-clone#example-MarkAsScalar) for more details.

### Mark pointer type as opaque

Some pointer values are used as enumerable const values.

A well-known case is `elliptic.Curve`. In package `crypto/tls`, curve type of a certificate is checked by comparing values to pre-defined curve values, e.g. `elliptic.P521()`. In this case, the curve values, which are pointers or structs, cannot be cloned deeply.

Currently, following types are marked as scalar by default.

- `elliptic.Curve`, which is `*elliptic.CurveParam` or `elliptic.p256Curve`.
- `reflect.Type`, which is `*reflect.rtype` defined in `runtime`.

If there is any pointer type defined in built-in package should be considered as opaque, please open new issue to let me know.
I will update the default.

If there is any custom pointer type should be considered as opaque, call `MarkAsOpaquePointer` to mark it manually. See [MarkAsOpaquePointer sample code](https://pkg.go.dev/github.com/huandu/go-clone#example-MarkAsOpaquePointer) for more details.

### Clone "no-copy" types defined in `sync` and `sync/atomic`

There are some "no-copy" types like `sync.Mutex`, `atomic.Value`, etc.
They cannot be cloned by copying all fields one by one, but we can alloc a new zero value and call methods to do proper initialization.

Currently, all "no-copy" types defined in `sync` and `sync/atomic` can be cloned properly using following strategies.

- `sync.Mutex`: Cloned value is a newly allocated zero mutex.
- `sync.RWMutex`: Cloned value is a newly allocated zero mutex.
- `sync.WaitGroup`: Cloned value is a newly allocated zero wait group.
- `sync.Cond`: Cloned value is a cond with a newly allocated zero lock.
- `sync.Pool`: Cloned value is an empty pool with the same `New` function.
- `sync.Map`: Cloned value is a sync map with cloned key/value pairs.
- `sync.Once`: Cloned value is a once type with the same done flag.
- `atomic.Value`/`atomic.Bool`/`atomic.Int32`/`atomic.Int64`/`atomic.Uint32`/`atomic.Uint64`/`atomic.Uintptr`: Cloned value is a new atomic value with the same value.

If there is any type defined in built-in package should be considered as "no-copy" types, please open new issue to let me know.
I will update the default.

### Set custom clone functions

If default clone strategy doesn't work for a struct type, we can call `SetCustomFunc` to register a custom clone function.

```go
SetCustomFunc(reflect.TypeOf(MyType{}), func(allocator *Allocator, old, new reflect.Value) {
    // Customized logic to copy the old to the new.
    // The old's type is MyType.
    // The new is a zero value of MyType and new.CanAddr() always returns true.
})
```

We can use `allocator` to clone any value or allocate new memory.
It's allowed to call `allocator.Clone` or `allocator.CloneSlowly` on `old` to clone its struct fields in depth without worrying about dead loop.

See [SetCustomFunc sample code](https://pkg.go.dev/github.com/huandu/go-clone#example-SetCustomFunc) for more details.

### Clone `atomic.Pointer[T]`

As there is no way to predefine a custom clone function for generic type `atomic.Pointer[T]`, cloning such atomic type is not supported by default. If we want to support it, we need to register a custom clone function manually.

Suppose we instantiate `atomic.Pointer[T]` with type `MyType1` and `MyType2` in a project, and then we can register custom clone functions like following.

```go
import "github.com/huandu/go-clone/generic"

func init() {
    // Register all instantiated atomic.Pointer[T] types in this project.
    clone.RegisterAtomicPointer[MyType1]()
    clone.RegisterAtomicPointer[MyType2]()
}
```

### `Wrap`, `Unwrap` and `Undo`

Package `clone` provides `Wrap`/`Unwrap` functions to protect a pointer value from any unexpected mutation.
It's useful when we want to protect a variable which should be immutable by design,
e.g. global config, the value stored in context, the value sent to a chan, etc.

```go
// Suppose we have a type T defined as following.
//     type T struct {
//         Foo int
//     }
v := &T{
    Foo: 123,
}
w := Wrap(v).(*T) // Wrap value to protect it.

// Use w freely. The type of w is the same as that of v.

// It's OK to modify w. The change will not affect v.
w.Foo = 456
fmt.Println(w.Foo) // 456
fmt.Println(v.Foo) // 123

// Once we need the original value stored in w, call `Unwrap`.
orig := Unwrap(w).(*T)
fmt.Println(orig == v) // true
fmt.Println(orig.Foo)  // 123

// Or, we can simply undo any change made in w.
// Note that `Undo` is significantly slower than `Unwrap`, thus
// the latter is always preferred.
Undo(w)
fmt.Println(w.Foo) // 123
```

## Performance

Here is the performance data running on my dev machine.

```text
go 1.20.1
goos: darwin
goarch: amd64
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
BenchmarkSimpleClone-12       7164530        156.7 ns/op       24 B/op        1 allocs/op
BenchmarkComplexClone-12       628056         1871 ns/op     1488 B/op       21 allocs/op
BenchmarkUnwrap-12           15498139        78.02 ns/op        0 B/op        0 allocs/op
BenchmarkSimpleWrap-12        3882360        309.7 ns/op       72 B/op        2 allocs/op
BenchmarkComplexWrap-12        949654         1245 ns/op      736 B/op       15 allocs/op
```

## License

This package is licensed under MIT license. See LICENSE for details.
