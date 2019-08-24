---
title: Policy Cheatsheet
navtitle: Policy Cheatsheet
kind: documentation
weight: 5
toc: true
---

## Assignment and Equality

```live:assign_equality:query:read_only
# assign variable x to value of field foo.bar.baz in input
x := input.foo.bar.baz

# check if variable x has same value as variable y
x == y

# check if variable x is a set containing "foo" and "bar"
x == {"foo", "bar"}

# OR

{"foo", "bar"} == x
```

## Lookup

#### Arrays

```live:lookup/arrays:query:read_only
# lookup value at index 0
val := arr[0]

 # check if value at index 0 is "foo"
"foo" == arr[0]

# find all indices i that have value "foo"
"foo" == arr[i]

# lookup last value
val := arr[count(arr)-1]
```

#### Objects

```live:lookup/objects:query:read_only
# lookup value for key "foo"
val := obj["foo"]

# check if value for key "foo" is "bar"
"bar" == obj["foo"]

# OR

"bar" == obj.foo

# check if key "foo" exists and is not false
obj.foo

# check if key assigned to variable k exists
k := "foo"
obj[k]

# check if path foo.bar.baz exists and is not false
obj.foo.bar.baz

# check if path foo.bar.baz, foo.bar, or foo does not exist or is false
not obj.foo.bar.bar
```

#### Sets

```live:lookup/sets:query:read_only
# check if "foo" belongs to the set
a_set["foo"]

# check if "foo" DOES NOT belong to the set
not a_set["foo"]

# check if the array ["a", "b", "c"] belongs to the set
a_set[["a", "b", "c"]]

# find all arrays of the form [x, "b", z] in the set
a_set[[x, "b", z]]
```

## Iteration

#### Arrays

```live:iteration/arrays:query:read_only
# iterate over indices i
arr[i]

# iterate over values
val := arr[_]

# iterate over index/value pairs
val := arr[i]
```

#### Objects

```live:iteration/objects:query:read_only
# iterate over keys
obj[key]

# iterate over values
val := obj[_]

# iterate over key/value pairs
val := obj[key]
```

#### Sets

```live:iteration/sets:query:read_only
# iterate over values
set[val]
```

#### Advanced

```live:iteration/advanced:query:read_only
# nested: find key k whose bar.baz array index i is 7
foo[k].bar.baz[i] == 7

# simultaneous: find keys in objects foo and bar with same value
foo[k1] == bar[k2]

# simultaneous self: find 2 keys in object foo with same value
foo[k1] == foo[k2]; k1 != k2

# multiple conditions: k has same value in both conditions
foo[k].bar.baz[i] == 7; foo[k].qux > 3
```

#### For All

```live:iteration/forall:query:read_only
# assert no values in set match predicate
count({x | set[x]; f(x)}) == 0

# assert all values in set make function f true
count({x | set[x]; f(x)}) == count(set)

# assert no values in set make function f true (using negation and helper rule)
not any_match

# assert all values in set make function f true (using negation and helper rule)
not any_not_match
```

```live:iteration/forall:module:read_only
any_match {
    set[x]
    f(x)
}

any_not_match {
    set[x]
    not f(x)
}
```

## Rules

In the examples below `...` represents one or more conditions.

#### Constants

```live:rules/constants:module:read_only
a = {1, 2, 3}
b = {4, 5, 6}
c = a | b
```

#### Conditionals (Boolean)

```live:rules/condbool:module:read_only
# p is true if ...
p = true { ...}

# OR

p { ... }
```

#### Conditionals

```live:rules/cond:module:read_only
default a = 1
a = 5 { ... }
a = 100 { ... }
```

#### Incremental

```live:rules/incremental:module:read_only
# a_set will contain values of x and values of y
a_set[x] { ... }
a_set[y] { ... }

# a_map will contain key->value pairs x->y and w->z
a_map[x] = y { ... }
a_map[w] = z { ... }
```

#### Ordered (Else)

```live:rules/ordered:module:read_only
default a = 1
a = 5 { ... }
else = 10 { ... }
```

#### Functions (Boolean)

```live:rules/funcs:module:read_only
f(x, y) {
    ...
}

# OR

f(x, y) = true {
    ...
}
```

#### Functions (Conditionals)

```live:rules/condfuncs:module:read_only
f(x) = "A" { x >= 90 }
f(x) = "B" { x >= 80; x < 90 }
f(x) = "C" { x >= 70; x < 80 }
```

## Patterns

#### Merge Objects

```live:rules/merge:query:read_only
x := {"a": true, "b": false}
y := {"b": "foo", "c": 4}
z := {"a": true, "b": "foo", "c": 4}
merge_objects(x, y) == z
```

```live:rules/merge:module:read_only
has_key(x, k) { _ = x[k] }

pick_first(k, a, b) = a[k]
pick_first(k, a, b) = b[k] { not has_key(a, k) }

merge_objects(a, b) = c {
    ks := {k | some k; _ = a[k]} | {k | some k; _ = b[k]}
    c := {k: v | some k; ks[k]; v := pick_first(k, b, a)}
}
```

## Tests

```live:tests:module:read_only
# define a rule that starts with test_
test_NAME { ... }

# override input.foo value using the 'with' keyword
data.foo.bar.deny with input.foo as {"bar": [1,2,3]}}
```