---
title: Language Cheatsheet
navtitle: Language Cheatsheet
kind: documentation
weight: 5
toc: true
---

## Assignment and Equality

```ruby
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

```ruby
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

```ruby
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

```ruby
# check if "foo" belongs to the set
a_set["foo"]

# check if the array ["a", "b", "c"] belongs to the set
a_set[["a", "b", "c"]]

# find all arrays of the form [x, "b", z] in the set
a_set[[x, "b", z]]
```

## Iteration

#### Arrays

```ruby
# iterate over indices i
arr[i]

# iterate over values
val := arr[_]

# iterate over index/value pairs
val := arr[i]
```

#### Objects

```ruby
# iterate over keys
obj[key]

# iterate over values
val := obj[_]

# iterate over key/value pairs
val := obj[key]
```

#### Sets

```ruby
# iterate over values
set[val]
```

#### Advanced

```ruby
# nested: find key k whose bar.baz array index i is 7
foo[k].bar.baz[i] == 7

# simultaneous: find keys in objects foo and bar with same value
foo[k1] == bar[k2]

# simultaneous self: find 2 keys in object foo with same value
foo[k1] == foo[k2]; k1 != k2

# multiple conditions: k has same value in both conditions
foo[k].bar.baz[i] == 7; foo[k].qux > 3
```

## Rules

In the examples below `...` represents one or more conditions.

#### Constants

```ruby
a = {1, 2, 3}
b = {4, 5, 6}
c = a | b
```

#### Conditionals (Boolean)

```ruby
# p is true if ...
p = true { ...}

# OR

p { ... }
```

#### Conditionals

```ruby
default a = 1
a = 5 { ... }
a = 100 { ... }
```

#### Incremental

```ruby
# a_set will contain values of x and values of y
a_set[x] { ... }
a_set[y] { ... }

# a_map will contain key->value pairs x->y and w->z
a_map[x] = y { ... }
a_map[w] = z { ... }
```

#### Ordered (Else)

```ruby
default a = 1
a = 5 { ... }
else = 10 { ... }
```

#### Functions (Boolean)

```ruby
f(x, y) {
    ...
}

# OR

f(x, y) = true {
    ...
}
```

#### Functions (Conditionals)

```ruby
f(x) = "A" { x >= 90 }
f(x) = "B" { x >= 80; x < 90 }
f(x) = "C" { x >= 70; x < 80 }
```

## Tests

```ruby
# define a rule that starts with test_
test_NAME { ... }

# override input.foo value using the 'with' keyword
data.foo.bar.deny with input.foo as {"bar": [1,2,3]}}
```