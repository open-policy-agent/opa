---
title: Policy Cheatsheet
navtitle: Cheatsheet
kind: best-practices
weight: 2
---

## Equality
```ruby
# Outside of a rule, assign variable `whitelist` to the set `hooli.com` and `initech.com`
whitelist = {"hooli.com", "initech.com"}
# Inside of a rule, assign LOCAL variable `x` to the value of label `costcenter`
x := input.request.metadata.labels.costcenter
# Check if variable `x` and `y` have the same value
x == y
# Check if variable `whitelist` has value {"hooli.com", "initech.com"}
whitelist == {"hooli.com", "initech.com"}
    # OR
{"hooli.com", "initech.com"} == whitelist
```

## Lookup
```ruby
# Array: lookup value at index 0
val := arr[0]
# Array: lookup all indexes with value "foo"
"foo" == arr[i]
# Array: check if index 0 has value "foo"
"foo" == arr[0]
# Array: last element
val := arr[count(arr) - 1]

# Object: lookup value for key "foo"
val := obj["foo"]
# Object: lookup keys for value "bar"
"bar" == obj[k]
# Object: check if key "foo" has value "bar"
"bar" == obj["foo"]  OR  "bar" == obj.foo
# Object: check if key "foo" exists
obj.foo
# Object: check if key assigned to variable k exists
k := "foo"
obj[k]
# Object: check if path foo.bar.baz exists
obj.foo.bar.baz
# Object: check if path foo.bar.baz, foo.bar, or foo does NOT exist
not obj.foo.bar.baz

# Set: check if "foo" belongs to the set
sett["foo"]
# Set: check if [1,2,3] belongs to the set
sett[[1,2,3]]
# Set: find all arrays of the form [x,2,x] in the set
sett[[x,2,x]]
```


## Basic Iteration
```ruby
# Arrays: iterate over indexes
arr[i]
# Arrays: iterate over values
val := arr[_]
# Arrays: iterate over index/value pairs
val := arr[i]
# Objects: iterate over keys
obj[k]
# Objects: iterate over values
val := obj[_]
# Objects: iterate over key/value pairs
val := obj[k]
# Sets: iterate over values
sett[val]
```

## Advanced Iteration
```ruby
# Nested iteration: Find key k whose bar.baz array index i is 7
foo[k].bar.baz[i] == 7
# Simultaneous iteration: Find keys in 2 objects with same value
foo[k1] == bar[k2]
# Simultaneous self iteration: find 2 keys with same value in 1 obj
foo[k1] == foo[k2]
# Simultaneous, nested iteration
foo[k1].foo[i1] == bar[k2].bar.baz[i2][j2]
# Multi-condition: k has same value in both conditions
foo[k].bar.baz[i] == 7
foo[k].qux > 3
```

## Rule heads
```ruby
# Constant:
foo = {1,2,3}
bar = {4,5,6}
# Constant: derived
baz = foo | bar
# Constant: conditional
a = 4 { <condition1> }
a = 57 { <condition2> }
a = 60 { <condition3> }
# Constant: conditional boolean
p { ... }  OR  p = true { ... }
# Function: conditional
p(x) = "A" { x >= 90 }
p(x) = "B" { x >= 80; x < 90}
p(x) = "C" { x >= 70; x < 80}
# Function: boolean
p(x) { ... }  OR p(x) = true { ... }
# Set: comprehension
p = { x | ... }
# Set: partial
p[x] { <condition1> }
p[y] { <condition2> }
# Object: comprehension
p = {x:y | ... }
# Object: partial
p[x] = y { <condition1> }
p[w] = z { <condition2> }
```

## Testing
```ruby
# Create a test
test_NAME { ... }
# evaluate `deny` within package `abc.def`
#    using `{"foo": "bar"}` for input
data.abc.def.deny with input as {"foo": "bar"}
```
