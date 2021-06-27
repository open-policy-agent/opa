---
title: Policy Reference
kind: documentation
weight: 3
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

### Arrays

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

### Objects

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
not obj.foo.bar.baz
```

### Sets

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

### Arrays

```live:iteration/arrays:query:read_only
# iterate over indices i
arr[i]

# iterate over values
val := arr[_]

# iterate over index/value pairs
val := arr[i]
```

### Objects

```live:iteration/objects:query:read_only
# iterate over keys
obj[key]

# iterate over values
val := obj[_]

# iterate over key/value pairs
val := obj[key]
```

### Sets

```live:iteration/sets:query:read_only
# iterate over values
set[val]
```

### Advanced

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

## For All

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

### Constants

```live:rules/constants:module:read_only
a = {1, 2, 3}
b = {4, 5, 6}
c = a | b
```

### Conditionals (Boolean)

```live:rules/condbool:module:read_only
# p is true if ...
p = true { ...}

# OR

p { ... }
```

### Conditionals

```live:rules/cond:module:read_only
default a = 1
a = 5 { ... }
a = 100 { ... }
```

### Incremental

```live:rules/incremental:module:read_only
# a_set will contain values of x and values of y
a_set[x] { ... }
a_set[y] { ... }

# a_map will contain key->value pairs x->y and w->z
a_map[x] = y { ... }
a_map[w] = z { ... }
```

### Ordered (Else)

```live:rules/ordered:module:read_only
default a = 1
a = 5 { ... }
else = 10 { ... }
```

### Functions (Boolean)

```live:rules/funcs:module:read_only
f(x, y) {
    ...
}

# OR

f(x, y) = true {
    ...
}
```

### Functions (Conditionals)

```live:rules/condfuncs:module:read_only
f(x) = "A" { x >= 90 }
f(x) = "B" { x >= 80; x < 90 }
f(x) = "C" { x >= 70; x < 80 }
```

## Tests

```live:tests:module:read_only
# define a rule that starts with test_
test_NAME { ... }

# override input.foo value using the 'with' keyword
data.foo.bar.deny with input.foo as {"bar": [1,2,3]}}
```

## Built-in Functions

The built-in functions for the language provide basic operations to manipulate
scalar values (e.g. numbers and strings), and aggregate functions that summarize
complex types.

### Comparison

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``x == y``</span>   | ``x`` is equal to ``y`` | ✅  |
| <span class="opa-keep-it-together">``x != y``</span>   | ``x`` is not equal to ``y`` | ✅  |
| <span class="opa-keep-it-together">``x < y``</span>   | ``x`` is less than ``y`` | ✅   |
| <span class="opa-keep-it-together">``x <= y``</span>   | ``x`` is less than or equal to ``y`` | ✅  |
| <span class="opa-keep-it-together">``x > y``</span>   | ``x`` is greater than ``y`` | ✅  |
| <span class="opa-keep-it-together">``x >= y``</span>   | ``x`` is greater than or equal to ``y`` | ✅ |

### Numbers

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``z := x + y``</span>   | ``z`` is the sum of ``x`` and ``y`` | ✅ |
| <span class="opa-keep-it-together">``z := x - y``</span>  | ``z`` is the difference of ``x`` and ``y`` | ✅ |
| <span class="opa-keep-it-together">``z := x * y``</span>   | ``z`` is the product of ``x`` and ``y`` | ✅ |
| <span class="opa-keep-it-together">``z := x / y``</span>   | ``z`` is the quotient of ``x`` and ``y``  | ✅ |
| <span class="opa-keep-it-together">``z := x % y``</span>   | ``z`` is the remainder from the division of ``x`` and ``y``  | ✅ |
| <span class="opa-keep-it-together">``output := round(x)``</span>    | ``output`` is ``x`` rounded to the nearest integer | ✅ |
| <span class="opa-keep-it-together">``output := ceil(x)``</span>    | ``output`` is ``x`` rounded up to the nearest integer | ✅ |
| <span class="opa-keep-it-together">``output := floor(x)``</span>    | ``output`` is ``x`` rounded down the nearest integer | ✅ |
| <span class="opa-keep-it-together">``output := abs(x)``</span>    | ``output`` is the absolute value of ``x`` | ✅ |
| <span class="opa-keep-it-together">``output := numbers.range(a, b)``</span> | ``output`` is the range of integer numbers between ``a`` and ``b`` (inclusive). If ``a`` == ``b`` then ``output`` == ``[a]``. If ``a`` < ``b`` the range is in ascending order. If ``a`` > ``b`` the range is in descending order. | ✅ |

### Aggregates

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := count(collection_or_string)``</span> | ``output`` is the length of the object, array, set, or string provided as input | ✅ |
| <span class="opa-keep-it-together">``output := sum(array_or_set)``</span> | ``output`` is the sum of the numbers in ``array_or_set`` | ✅ |
| <span class="opa-keep-it-together">``output := product(array_or_set)``</span> | ``output`` is the product of the numbers in ``array_or_set`` | ✅ |
| <span class="opa-keep-it-together">``output := max(array_or_set)``</span> | ``output`` is the maximum value in ``array_or_set`` | ✅ |
| <span class="opa-keep-it-together">``output := min(array_or_set)``</span> | ``output`` is the minimum value in ``array_or_set`` | ✅ |
| <span class="opa-keep-it-together">``output := sort(array_or_set)``</span> | ``output`` is the sorted ``array`` containing elements from ``array_or_set``. | ✅ |

### Arrays
| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := array.concat(array, array)``</span> | ``output`` is the result of concatenating the two input arrays together. | ✅ |
  <span class="opa-keep-it-together">``output := array.slice(array, startIndex, stopIndex)``</span> | ``output`` is the part of the ``array`` from ``startIndex`` to ``stopIndex`` including the first but excluding the last. If `startIndex >= stopIndex` then `output == []`. If both `startIndex` and `stopIndex` are less than zero, `output == []`. Otherwise, `startIndex` and `stopIndex` are clamped to 0 and `count(array)` respectively. | ✅ |

### Sets

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``s3 := s1 & s2``</span> | ``s3`` is the intersection of ``s1`` and ``s2``. | ✅ |
| <span class="opa-keep-it-together"><code>s3 := s1 &#124; s2</code></span> | ``s3`` is the union of ``s1`` and ``s2``. | ✅ |
| <span class="opa-keep-it-together">``s3 := s1 - s2``</span> | ``s3`` is the difference between ``s1`` and ``s2``, i.e., the elements in ``s1`` that are not in ``s2`` | ✅ |
| <span class="opa-keep-it-together">``output := intersection(set[set])``</span> | ``output`` is the intersection of the sets in the input set | ✅ |
| <span class="opa-keep-it-together">``output := union(set[set])``</span> | ``output`` is the union of the sets in the input set  | ✅ |

### Objects

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">`value := object.get(object, key, default)`</span> | `value` is the value stored by the `object` at `key`. If no value is found, `default` is returned. | ✅ |
| <span class="opa-keep-it-together">`output := object.remove(object, keys)`</span> | `output` is a new object which is the result of removing the specified `keys` from `object`. `keys` must be either an array, object, or set of keys. | ✅ |
| <span class="opa-keep-it-together">`output := object.union(objectA, objectB)`</span> | `output` is a new object which is the result of an asymmetric recursive union of two objects where conflicts are resolved by choosing the key from the right-hand object (`objectB`). For example: `object.union({"a": 1, "b": 2, "c": {"d": 3}}, {"a": 7, "c": {"d": 4, "e": 5}})` will result in `{"a": 7, "b": 2, "c": {"d": 4, "e": 5}}`  | ✅ |
| <span class="opa-keep-it-together">`filtered := object.filter(object, keys)`</span> | `filtered` is a new object with the remaining data from `object` with only keys specified in `keys` which is an array, object, or set of keys. For example: `object.filter({"a": {"b": "x", "c": "y"}, "d": "z"}, ["a"])` will result in `{"a": {"b": "x", "c": "y"}}`). | ✅ |
| <span class="opa-keep-it-together">`filtered := json.filter(object, paths)`</span> | `filtered` is the remaining data from `object` with only keys specified in `paths` which is an array or set of JSON string paths. For example: `json.filter({"a": {"b": "x", "c": "y"}}, ["a/b"])` will result in `{"a": {"b": "x"}}`). Paths are not filtered in-order and are deduplicated before being evaluated. | ✅ |
| <span class="opa-keep-it-together">`output := json.remove(object, paths)`</span> | `output` is a new object which is the result of removing all keys specified in `paths` which is an array or set of JSON string paths. For example: `json.remove({"a": {"b": "x", "c": "y"}}, ["a/b"])` will result in `{"a": {"c": "y"}}`. Paths are not removed in-order and are deduplicated before being evaluated. | ✅ |
| <span class="opa-keep-it-together">`output := json.patch(object, patches)`</span> | `output` is a the object obtained after consecutively applying all [JSON Patch](https://tools.ietf.org/html/rfc6902) operations in the array `patches`. For example: `json.patch({"a": {"foo": 1}}, [{"op": "add", "path": "/a/bar", "value": 2}])` results in `{"a": {"foo": 1, "bar": 2}`.  The patches are applied atomically: if any of them fails, the result will be undefined. | ``SDK-dependent`` |

* When `keys` are provided as an object only the top level keys on the object will be used, values are ignored.
  For example: `object.remove({"a": {"b": {"c": 2}}, "x": 123}, {"a": 1}) == {"x": 123}` regardless of the value
  for key `a` in the keys object, the following `keys` object gives the same result
  `object.remove({"a": {"b": {"c": 2}}, "x": 123}, {"a": {"b": {"foo": "bar"}}}) == {"x": 123}`.


* The `json` string `paths` may reference into array values by using index numbers. For example with the object
  `{"a": ["x", "y", "z"]}` the path `a/1` references `y`. Nested structures are supported as well, for example:
  `{"a": ["x", {"y": {"y1": {"y2": ["foo", "bar"]}}}, "z"]}` the path `a/1/y1/y2/0` references `"foo"`.


* The `json` string `paths` support `~0`, or `~1` characters for `~` and `/` characters in key names.
  It does not support `-` for last index of an array. For example the path `/foo~1bar~0` will reference `baz`
  in `{ "foo/bar~": "baz" }`.


* The `json` string `paths` may be an array of string path segments rather than a `/` separated string. For example
  the path `a/b/c` can be passed in as `["a", "b", "c"]`.


### Strings

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := concat(delimiter, array_or_set)``</span> | ``output`` is the result of joining together the elements of ``array_or_set`` with the  string ``delimiter`` | ✅ |
| <span class="opa-keep-it-together">``contains(string, search)``</span> | true if ``string`` contains ``search`` | ✅ |
| <span class="opa-keep-it-together">``endswith(string, search)``</span> | true if ``string`` ends with ``search`` | ✅ |
| <span class="opa-keep-it-together">``output := format_int(number, base)``</span> | ``output`` is string representation of ``number`` in the given ``base`` | ✅ |
| <span class="opa-keep-it-together">``output := indexof(string, search)``</span> | ``output`` is the index inside ``string`` where ``search`` first occurs, or -1 if ``search`` does not exist | ✅ |
| <span class="opa-keep-it-together">``output := lower(string)``</span> | ``output`` is ``string`` after converting to lower case | ✅ |
| <span class="opa-keep-it-together">``output := replace(string, old, new)``</span> | ``output`` is a ``string`` representing ``string`` with all instances of ``old`` replaced by ``new`` | ✅ |
| <span class="opa-keep-it-together">``output := strings.replace_n(patterns, string)``</span> | ``patterns`` is an object with old, new string key value pairs (e.g. ``{"old1": "new1", "old2": "new2", ...}``). ``output`` is a ``string`` with all old strings inside ``patterns`` replaced by the new strings | ✅ |
| <span class="opa-keep-it-together">``output := split(string, delimiter)``</span> | ``output`` is ``array[string]`` representing elements of ``string`` separated by ``delimiter`` | ✅ |
| <span class="opa-keep-it-together">``output := sprintf(string, values)``</span> | ``output`` is a ``string`` representing ``string`` formatted by the values in the ``array`` ``values``. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``startswith(string, search)``</span> | true if ``string`` begins with ``search`` | ✅ |
| <span class="opa-keep-it-together">``output := substring(string, start, length)``</span> | ``output`` is the portion of ``string`` from index ``start`` and having a length of ``length``.  If ``length`` is less than zero, ``length`` is the remainder of the ``string``. If ``start`` is greater than the length of the string, ``output`` is empty. It is invalid to pass a negative offset to this function. | ✅ |
| <span class="opa-keep-it-together">``output := trim(string, cutset)``</span> | ``output`` is a ``string`` representing ``string`` with all leading and trailing instances of the characters in ``cutset`` removed. | ✅ |
| <span class="opa-keep-it-together">``output := trim_left(string, cutset)``</span> | ``output`` is a ``string`` representing ``string`` with all leading instances of the characters in ``cutset`` removed. | ✅ |
| <span class="opa-keep-it-together">``output := trim_prefix(string, prefix)``</span> | ``output`` is a ``string`` representing ``string`` with leading instance of ``prefix`` removed. If ``string`` doesn't start with prefix, ``string`` is returned unchanged.| ✅ |
| <span class="opa-keep-it-together">``output := trim_right(string, cutset)``</span> | ``output`` is a ``string`` representing ``string`` with all trailing instances of the characters in ``cutset`` removed. | ✅ |
| <span class="opa-keep-it-together">``output := trim_suffix(string, suffix)``</span> | ``output`` is a ``string`` representing ``string`` with trailing instance of ``suffix`` removed. If ``string`` doesn't end with suffix, ``string`` is returned unchanged.| ✅ |
| <span class="opa-keep-it-together">``output := trim_space(string)``</span> | ``output`` is a ``string`` representing ``string`` with all leading and trailing white space removed.| ✅ |
| <span class="opa-keep-it-together">``output := upper(string)``</span> | ``output`` is ``string`` after converting to upper case | ✅ |

### Regex
| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := regex.match(pattern, value)``</span> | ``output`` is a ``boolean`` that indicates if ``value`` matches the regex ``pattern``. | ✅ |
| <span class="opa-keep-it-together">``output := regex.is_valid(pattern)``</span> | ``output`` is a ``boolean`` that indicates if ``pattern` is a valid regex pattern. The detailed syntax for regex patterns is defined by https://github.com/google/re2/wiki/Syntax. | ✅ |
| <span class="opa-keep-it-together">``output := regex.split(pattern, string)``</span> | ``output`` is ``array[string]`` representing elements of ``string`` separated by ``pattern`` | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``regex.globs_match(glob1, glob2)``</span> | true if the intersection of regex-style globs ``glob1`` and ``glob2`` matches a non-empty set of non-empty strings. The set of regex symbols is limited for this builtin: only ``.``, ``*``, ``+``, ``[``, ``-``, ``]`` and ``\`` are treated as special symbols. | ``SDK-dependent`` |
| <span class="opa-keep-it-normal">``output := regex.template_match(pattern, string, delimiter_start, delimiter_end)``</span> | ``output`` is true if ``string`` matches ``pattern``. ``pattern`` is a string containing ``0..n`` regular expressions delimited by ``delimiter_start`` and ``delimiter_end``. Example ``regex.template_match("urn:foo:{.*}", "urn:foo:bar:baz", "{", "}")`` returns ``true``. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := regex.find_n(pattern, string, number)``</span> | ``output`` is an ``array[string]`` with the ``number`` of values matching the ``pattern``. A ``number`` of ``-1`` means all matches. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := regex.find_all_string_submatch_n(pattern, string, number)``</span> | ``output`` is an ``array[array[string]]`` with the outer `array` including a ``number`` of matches which match the ``pattern``. A ``number`` of ``-1`` means all matches. | ✅ |

### Glob
| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := glob.match(pattern, delimiters, match)``</span> | ``output`` is true if ``match`` can be found in ``pattern`` which is separated by ``delimiters``. For valid patterns, check the table below. Argument ``delimiters`` is an array of single-characters (e.g. `[".", ":"]`). If ``delimiters`` is empty, it defaults to ``["."]``. | ✅ |
| <span class="opa-keep-it-together">``output := glob.quote_meta(pattern)``</span> | ``output`` is the escaped string of ``pattern``. Calling ``glob.quote_meta("*.github.com", output)`` returns ``\\*.github.com`` as ``output``. | ``SDK-dependent`` |

The following table shows examples of how ``glob.match`` works:

| ``call`` | ``output`` | Description |
| -------- | ---------- | ----------- |
| ``output := glob.match("*.github.com", [], "api.github.com")`` | ``true`` | A glob with the default ``["."]`` delimiter. |
| ``output := glob.match("*.github.com", [], "api.cdn.github.com")`` | ``false`` | A glob with the default ``["."]`` delimiter. |
| ``output := glob.match("*:github:com", [":"], "api:github:com")`` | ``true`` | A glob with delimiters ``[":"]``. |
| ``output := glob.match("api.**.com", [], "api.github.com")`` | ``true`` | A super glob. |
| ``output := glob.match("api.**.com", [], "api.cdn.github.com")`` | ``true`` | A super glob. |
| ``output := glob.match("?at", [], "cat")`` | ``true`` | A glob with a single character wildcard. |
| ``output := glob.match("?at", [], "at")`` | ``false`` | A glob with a single character wildcard. |
| ``output := glob.match("[abc]at", [], "bat")`` | ``true`` | A glob with character-list matchers. |
| ``output := glob.match("[abc]at", [], "cat")`` | ``true`` | A glob with character-list matchers. |
| ``output := glob.match("[abc]at", [], "lat")`` | ``false`` | A glob with character-list matchers. |
| ``output := glob.match("[!abc]at", [], "cat")`` | ``false`` | A glob with negated character-list matchers. |
| ``output := glob.match("[!abc]at", [], "lat")`` | ``true`` | A glob with negated character-list matchers. |
| ``output := glob.match("[a-c]at", [], "cat")`` | ``true`` | A glob with character-range matchers. |
| ``output := glob.match("[a-c]at", [], "lat")`` | ``false`` | A glob with character-range matchers. |
| ``output := glob.match("[!a-c]at", [], "cat")`` | ``false`` | A glob with negated character-range matchers. |
| ``output := glob.match("[!a-c]at", [], "lat")`` | ``true`` | A glob with negated character-range matchers. |
| ``output := glob.match("{cat,bat,[fr]at}", [], "cat")`` | ``true`` | A glob with pattern-alternatives matchers. |
| ``output := glob.match("{cat,bat,[fr]at}", [], "bat")`` | ``true`` | A glob with pattern-alternatives matchers. |
| ``output := glob.match("{cat,bat,[fr]at}", [], "rat")`` | ``true`` | A glob with pattern-alternatives matchers. |
| ``output := glob.match("{cat,bat,[fr]at}", [], "at")`` | ``false`` | A glob with pattern-alternatives matchers. |

### Bitwise

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``z := bits.or(x, y)``</span>  | ``z`` is the bitwise or of integers ``x`` and ``y`` | ✅ |
| <span class="opa-keep-it-together">``z := bits.and(x, y)``</span> | ``z`` is the bitwise and of integers ``x`` and ``y`` | ✅ |
| <span class="opa-keep-it-together">``z := bits.negate(x)``</span> | ``z`` is the bitwise negation (flip) of integer ``x`` | ✅ |
| <span class="opa-keep-it-together">``z := bits.xor(x, y)``</span> | ``z`` is the bitwise exclusive-or of integers ``x`` and ``y`` | ✅ |
| <span class="opa-keep-it-together">``z := bits.lsh(x, s)``</span> | ``z`` is the bitshift of integer ``x`` by ``s`` bits to the left | ✅ |
| <span class="opa-keep-it-together">``z := bits.rsh(x, s)``</span> | ``z`` is the bitshift of integer ``x`` by ``s`` bits to the right | ✅ |

### Conversions

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := to_number(x)``</span> | ``output`` is ``x`` converted to a number. `null` is converted to zero, `true` and `false` are converted to one and zero (respectively), `string` values are interpreted as base 10, and `numbers` are a no-op. Other types are not supported. | ✅ |

### Units

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := units.parse_bytes(x)``</span> | ``output`` is ``x`` converted to a number with support for standard byte units (e.g., KB, KiB, etc.) KB, MB, GB, and TB are treated as decimal units and KiB, MiB, GiB, and TiB are treated as binary units. The bytes symbol (b/B) in the unit is optional and omitting it wil give the same result (e.g. Mi and MiB) | ``SDK-dependent`` |

### Types

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := is_number(x)``</span> | ``output`` is ``true`` if ``x`` is a number; otherwise undefined| ✅ |
| <span class="opa-keep-it-together">``output := is_string(x)``</span> | ``output`` is ``true`` if ``x`` is a string; otherwise undefined | ✅ |
| <span class="opa-keep-it-together">``output := is_boolean(x)``</span> | ``output`` is ``true`` if ``x`` is a boolean; otherwise undefined | ✅ |
| <span class="opa-keep-it-together">``output := is_array(x)``</span> | ``output`` is ``true`` if ``x`` is an array; otherwise undefined | ✅ |
| <span class="opa-keep-it-together">``output := is_set(x)``</span> | ``output`` is ``true`` if ``x`` is a set; otherwise undefined | ✅ |
| <span class="opa-keep-it-together">``output := is_object(x)``</span> | ``output`` is ``true`` if ``x`` is an object; otherwise undefined | ✅ |
| <span class="opa-keep-it-together">``output := is_null(x)``</span> | ``output`` is ``true`` if ``x`` is null; otherwise undefined | ✅ |
| <span class="opa-keep-it-together">``output := type_name(x)``</span> | ``output`` is the type of ``x`` |

### Encoding

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := base64.encode(x)``</span> | ``output`` is ``x`` serialized to a base64 encoded string without padding | ✅ |
| <span class="opa-keep-it-together">``output := base64.decode(string)``</span> | ``output`` is ``x`` deserialized from a base64 encoding string without padding | ✅ |
| <span class="opa-keep-it-together">``output := base64url.encode(x)``</span> | ``output`` is ``x`` serialized to a base64url encoded string with padding | ✅ |
| <span class="opa-keep-it-together">``output := base64url.encode_no_pad(x)``</span> | ``output`` is ``x`` serialized to a base64url encoded string without padding | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := base64url.decode(string)``</span> | ``output`` is ``string`` deserialized from a base64url encoded string with or without padding | ✅ |
| <span class="opa-keep-it-together">``output := urlquery.encode(string)``</span> | ``output`` is ``string`` serialized to a URL query parameter encoded string | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := urlquery.encode_object(object)``</span> | ``output`` is ``object`` serialized to a URL query parameter encoded string | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := urlquery.decode(string)``</span> | ``output`` is ``string`` deserialized from a URL query parameter encoded string | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := urlquery.decode_object(string)``</span> | ``output`` is ``object`` deserialized from a URL query parameter string | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := json.marshal(x)``</span> | ``output`` is ``x`` serialized to a JSON string | ✅ |
| <span class="opa-keep-it-together">``output := json.unmarshal(string)``</span> | ``output`` is ``string`` deserialized to a term from a JSON encoded string | ✅ |
| <span class="opa-keep-it-together">``output := json.is_valid(string)``</span> | ``output`` is a ``boolean`` that indicated whether ``string`` is a valid JSON document | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := yaml.marshal(x)``</span> | ``output`` is ``x`` serialized to a YAML string | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := yaml.unmarshal(string)``</span> | ``output`` is ``string`` deserialized to a term from YAML encoded string | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := yaml.is_valid(string)``</span> | ``output`` is a ``boolean`` that indicated whether ``string`` is a valid YAML document that can be decoded by `yaml.unmarshal` | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := hex.encode(x)``</span> | ``output`` is ``x`` serialized to a hex encoded string | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := hex.decode(string)``</span> | ``output`` is a ``string`` deserialized from a hex encoded string | ``SDK-dependent`` |

### Token Signing

OPA provides two builtins that implement JSON Web Signature [RFC7515](https://tools.ietf.org/html/rfc7515) functionality.

``io.jwt.encode_sign_raw()`` takes three JSON Objects (strings) as parameters and returns their JWS Compact Serialization.
 This builtin should be used by those that want maximum control over the signing and serialization procedure. It is
 important to remember that StringOrURI values are compared as case-sensitive strings with no transformations or
 canonicalizations applied. Therefore, line breaks and whitespaces are significant.

``io.jwt.encode_sign()`` takes three Rego Objects as parameters and returns their JWS Compact Serialization. This builtin
 should be used by those that want to use rego objects for signing during policy evaluation.

> Note that with `io.jwt.encode_sign` the Rego objects are serialized to JSON with standard formatting applied
> whereas the `io.jwt.encode_sign_raw` built-in will **not** affect whitespace of the strings passed in.
> This will mean that the final encoded token may have different string values, but the decoded and parsed
> JSON will match.

The following algorithms are supported:

	ES256       "ES256" // ECDSA using P-256 and SHA-256
	ES384       "ES384" // ECDSA using P-384 and SHA-384
	ES512       "ES512" // ECDSA using P-521 and SHA-512
	HS256       "HS256" // HMAC using SHA-256
	HS384       "HS384" // HMAC using SHA-384
	HS512       "HS512" // HMAC using SHA-512
	PS256       "PS256" // RSASSA-PSS using SHA256 and MGF1-SHA256
	PS384       "PS384" // RSASSA-PSS using SHA384 and MGF1-SHA384
	PS512       "PS512" // RSASSA-PSS using SHA512 and MGF1-SHA512
	RS256       "RS256" // RSASSA-PKCS-v1.5 using SHA-256
	RS384       "RS384" // RSASSA-PKCS-v1.5 using SHA-384
	RS512       "RS512" // RSASSA-PKCS-v1.5 using SHA-512

<br>

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := io.jwt.encode_sign_raw(headers, payload, key)``</span> | ``headers``, ``payload`` and  ``key`` as strings that represent the JWS Protected Header, JWS Payload and JSON Web Key ([RFC7517](https://tools.ietf.org/html/rfc7517)) respectively.| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.encode_sign(headers, payload, key)``</span> | ``headers``, ``payload`` and  ``key`` are JSON objects that represent the JWS Protected Header, JWS Payload and JSON Web Key ([RFC7517](https://tools.ietf.org/html/rfc7517)) respectively.| ``SDK-dependent`` |

> Note that the key's provided should be base64 encoded (without padding) as per the specification ([RFC7517](https://tools.ietf.org/html/rfc7517)).
> This differs from the plain text secrets provided with the algorithm specific verify built-ins described below.

#### Token Signing Examples

```live:jwt:module:hidden
package jwt
```

##### Symmetric Key (HMAC with SHA-256)

```live:jwt/hs256:query:merge_down
io.jwt.encode_sign({
    "typ": "JWT",
    "alg": "HS256"
}, {
    "iss": "joe",
    "exp": 1300819380,
    "aud": ["bob", "saul"],
    "http://example.com/is_root": true,
    "privateParams": {
        "private_one": "one",
        "private_two": "two"
    }
}, {
    "kty": "oct",
    "k": "AyM1SysPpbyDfgZld3umj1qzKObwVMkoqQ-EstJQLr_T-1qS0gZH75aKtMN3Yj0iPS4hcgUuTwjAzZr1Z9CAow"
})
```
```live:jwt/hs256:output
```

##### Symmetric Key with empty JSON payload

```live:jwt/hs256_nopayload:query:merge_down
io.jwt.encode_sign({
    "typ": "JWT",
    "alg": "HS256"},
    {}, {
    "kty": "oct",
    "k": "AyM1SysPpbyDfgZld3umj1qzKObwVMkoqQ-EstJQLr_T-1qS0gZH75aKtMN3Yj0iPS4hcgUuTwjAzZr1Z9CAow"
})
```
```live:jwt/hs256_nopayload:output
```

##### RSA Key (RSA Signature with SHA-256)

```live:jwt/rs256:query:merge_down
io.jwt.encode_sign({
    "alg": "RS256"
}, {
    "iss": "joe",
    "exp": 1300819380,
    "aud": ["bob", "saul"],
    "http://example.com/is_root": true,
    "privateParams": {
        "private_one": "one",
        "private_two": "two"
    }
},
{
    "kty": "RSA",
    "n": "ofgWCuLjybRlzo0tZWJjNiuSfb4p4fAkd_wWJcyQoTbji9k0l8W26mPddxHmfHQp-Vaw-4qPCJrcS2mJPMEzP1Pt0Bm4d4QlL-yRT-SFd2lZS-pCgNMsD1W_YpRPEwOWvG6b32690r2jZ47soMZo9wGzjb_7OMg0LOL-bSf63kpaSHSXndS5z5rexMdbBYUsLA9e-KXBdQOS-UTo7WTBEMa2R2CapHg665xsmtdVMTBQY4uDZlxvb3qCo5ZwKh9kG4LT6_I5IhlJH7aGhyxXFvUK-DWNmoudF8NAco9_h9iaGNj8q2ethFkMLs91kzk2PAcDTW9gb54h4FRWyuXpoQ",
    "e": "AQAB",
    "d": "Eq5xpGnNCivDflJsRQBXHx1hdR1k6Ulwe2JZD50LpXyWPEAeP88vLNO97IjlA7_GQ5sLKMgvfTeXZx9SE-7YwVol2NXOoAJe46sui395IW_GO-pWJ1O0BkTGoVEn2bKVRUCgu-GjBVaYLU6f3l9kJfFNS3E0QbVdxzubSu3Mkqzjkn439X0M_V51gfpRLI9JYanrC4D4qAdGcopV_0ZHHzQlBjudU2QvXt4ehNYTCBr6XCLQUShb1juUO1ZdiYoFaFQT5Tw8bGUl_x_jTj3ccPDVZFD9pIuhLhBOneufuBiB4cS98l2SR_RQyGWSeWjnczT0QU91p1DhOVRuOopznQ",
    "p": "4BzEEOtIpmVdVEZNCqS7baC4crd0pqnRH_5IB3jw3bcxGn6QLvnEtfdUdiYrqBdss1l58BQ3KhooKeQTa9AB0Hw_Py5PJdTJNPY8cQn7ouZ2KKDcmnPGBY5t7yLc1QlQ5xHdwW1VhvKn-nXqhJTBgIPgtldC-KDV5z-y2XDwGUc",
    "q": "uQPEfgmVtjL0Uyyx88GZFF1fOunH3-7cepKmtH4pxhtCoHqpWmT8YAmZxaewHgHAjLYsp1ZSe7zFYHj7C6ul7TjeLQeZD_YwD66t62wDmpe_HlB-TnBA-njbglfIsRLtXlnDzQkv5dTltRJ11BKBBypeeF6689rjcJIDEz9RWdc",
    "dp": "BwKfV3Akq5_MFZDFZCnW-wzl-CCo83WoZvnLQwCTeDv8uzluRSnm71I3QCLdhrqE2e9YkxvuxdBfpT_PI7Yz-FOKnu1R6HsJeDCjn12Sk3vmAktV2zb34MCdy7cpdTh_YVr7tss2u6vneTwrA86rZtu5Mbr1C1XsmvkxHQAdYo0",
    "dq": "h_96-mK1R_7glhsum81dZxjTnYynPbZpHziZjeeHcXYsXaaMwkOlODsWa7I9xXDoRwbKgB719rrmI2oKr6N3Do9U0ajaHF-NKJnwgjMd2w9cjz3_-kyNlxAr2v4IKhGNpmM5iIgOS1VZnOZ68m6_pbLBSp3nssTdlqvd0tIiTHU",
    "qi": "IYd7DHOhrWvxkwPQsRM2tOgrjbcrfvtQJipd-DlcxyVuuM9sQLdgjVk2oy26F0EmpScGLq2MowX7fhd_QJQ3ydy5cY7YIBi87w93IKLEdfnbJtoOPLUW0ITrJReOgo1cq9SbsxYawBgfp_gh6A5603k2-ZQwVK0JKSHuLFkuQ3U"
})
```
```live:jwt/rs256:output
```

##### Raw Token Signing

If you need to generate the signature for a serialized token you an use the
`io.jwt.encode_sign_raw` built-in function which accepts JSON serialized string
parameters.

```live:jwt/raw:query:merge_down
io.jwt.encode_sign_raw(
    `{"typ":"JWT","alg":"HS256"}`,
     `{"iss":"joe","exp":1300819380,"http://example.com/is_root":true}`,
    `{"kty":"oct","k":"AyM1SysPpbyDfgZld3umj1qzKObwVMkoqQ-EstJQLr_T-1qS0gZH75aKtMN3Yj0iPS4hcgUuTwjAzZr1Z9CAow"}`
)
```
```live:jwt/raw:output
```

### Token Verification

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := io.jwt.verify_rs256(string, certificate)``</span> | ``output`` is ``true`` if the RS256 signature of the input token is valid. ``certificate`` is the PEM encoded certificate, PEM encoded public key, or the JWK key (set) used to verify the RS256 signature| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.verify_rs384(string, certificate)``</span> | ``output`` is ``true`` if the RS384 signature of the input token is valid. ``certificate`` is the PEM encoded certificate, PEM encoded public key, or the JWK key (set) used to verify the RS384 signature| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.verify_rs512(string, certificate)``</span> | ``output`` is ``true`` if the RS512 signature of the input token is valid. ``certificate`` is the PEM encoded certificate, PEM encoded public key, or the JWK key (set) used to verify the RS512 signature| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.verify_ps256(string, certificate)``</span> | ``output`` is ``true`` if the PS256 signature of the input token is valid. ``certificate`` is the PEM encoded certificate, PEM encoded public key or the JWK key (set) used to verify the PS256 signature| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.verify_ps384(string, certificate)``</span> | ``output`` is ``true`` if the PS384 signature of the input token is valid. ``certificate`` is the PEM encoded certificate, PEM encoded public key or the JWK key (set) used to verify the PS384 signature| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.verify_ps512(string, certificate)``</span> | ``output`` is ``true`` if the PS512 signature of the input token is valid. ``certificate`` is the PEM encoded certificate, PEM encoded public key or the JWK key (set) used to verify the PS512 signature| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.verify_es256(string, certificate)``</span> | ``output`` is ``true`` if the ES256 signature of the input token is valid. ``certificate`` is the PEM encoded certificate, PEM encoded public key or the JWK key (set) used to verify the ES256 signature| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.verify_es384(string, certificate)``</span> | ``output`` is ``true`` if the ES384 signature of the input token is valid. ``certificate`` is the PEM encoded certificate, PEM encoded public key or the JWK key (set) used to verify the ES384 signature| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.verify_es512(string, certificate)``</span> | ``output`` is ``true`` if the ES512 signature of the input token is valid. ``certificate`` is the PEM encoded certificate, PEM encoded public key or the JWK key (set) used to verify the ES512 signature| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.verify_hs256(string, secret)``</span> | ``output`` is ``true`` if the Secret signature of the input token is valid. ``secret`` is a plain text secret used to verify the HS256 signature| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.verify_hs384(string, secret)``</span> | ``output`` is ``true`` if the Secret signature of the input token is valid. ``secret`` is a plain text secret used to verify the HS384 signature| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.verify_hs512(string, secret)``</span> | ``output`` is ``true`` if the Secret signature of the input token is valid. ``secret`` is a plain text secret used to verify the HS512 signature| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.decode(string)``</span> | ``output`` is of the form ``[header, payload, sig]``.  ``header`` and ``payload`` are ``object``. ``sig`` is the hexadecimal representation of the signature on the token. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := io.jwt.decode_verify(string, constraints)``</span> | ``output`` is of the form ``[valid, header, payload]``.  If the input token verifies and meets the requirements of ``constraints`` then ``valid`` is ``true`` and ``header`` and ``payload`` are objects containing the JOSE header and the JWT claim set. Otherwise, ``valid`` is ``false`` and ``header`` and ``payload`` are ``{}``. Supports the following algorithms: HS256, HS384, HS512, RS256, RS384, RS512, ES256, ES384, ES512, PS256, PS384 and PS512. | ``SDK-dependent`` |

> Note that the `io.jwt.verify_XX` built-in methods verify **only** the signature. They **do not** provide any validation for the JWT
> payload and any claims specified. The `io.jwt.decode_verify` built-in will verify the payload and **all** standard claims.

The input `string` is a JSON Web Token encoded with JWS Compact Serialization. JWE and JWS JSON Serialization are not supported. If nested signing was used, the ``header``, ``payload`` and ``signature`` will represent the most deeply nested token.

For ``io.jwt.decode_verify``, ``constraints`` is an object with the following members:

| Name | Meaning | Required |
| ---- | ------- | -------- |
| ``cert`` | A PEM encoded certificate, PEM encoded public key, or a JWK key (set) containing an RSA or ECDSA public key. | See below |
| ``secret`` | The secret key for HS256, HS384 and HS512 verification. | See below |
| ``alg`` | The JWA algorithm name to use. If it is absent then any algorithm that is compatible with the key is accepted. | Optional |
| ``iss`` | The issuer string. If it is present the only tokens with this issuer are accepted. If it is absent then any issuer is accepted. | Optional |
| ``time`` | The time in nanoseconds to verify the token at. If this is present then the ``exp`` and ``nbf`` claims are compared against this value. If it is absent then they are compared against the current time. | Optional |
| ``aud`` | The audience that the verifier identifies with.  If this is present then the ``aud`` claim is checked against it. **If it is absent then the ``aud`` claim must be absent too.** | Optional |

Exactly one of ``cert`` and ``secret`` must be present. If there are any
unrecognized constraints then the token is considered invalid.


#### Token Verification Examples

The examples below use the following token:

```live:jwt/verify:module
es256_token = "eyJ0eXAiOiAiSldUIiwgImFsZyI6ICJFUzI1NiJ9.eyJuYmYiOiAxNDQ0NDc4NDAwLCAiaXNzIjogInh4eCJ9.lArczfN-pIL8oUU-7PU83u-zfXougXBZj6drFeKFsPEoVhy9WAyiZlRshYqjTSXdaw8yw2L-ovt4zTUZb2PWMg"
```

##### Using JWKS
This example shows a two-step process to verify the token signature and then decode it for
further checks of the payload content. This approach gives more flexibility in verifying only
the claims that the policy needs to enforce.
```live:jwt/verify/jwks:module
jwks = `{
    "keys": [{
        "kty":"EC",
        "crv":"P-256",
        "x":"z8J91ghFy5o6f2xZ4g8LsLH7u2wEpT2ntj8loahnlsE",
        "y":"7bdeXLH61KrGWRdh7ilnbcGQACxykaPKfmBccTHIOUo"
    }]
}`
```

```live:jwt/verify/jwks/two_step:query:merge_down
io.jwt.verify_es256(es256_token, jwks)                  # Verify the token with the JWKS
[header, payload, _] := io.jwt.decode(es256_token)      # Decode the token
payload.iss == "xxx"                                    # Ensure the issuer (`iss`) claim is the expected value
```
```live:jwt/verify/jwks/two_step:output
```
The next example shows doing the token signature verification, decoding, and content checks
all in one call using `io.jwt.decode_verify`. Note that this gives less flexibility in validating
the payload content as **all** claims defined in the JWT spec are verified with the provided
constraints.
```live:jwt/verify/jwks/one_step:query:merge_down
[valid, header, payload] := io.jwt.decode_verify(es256_token, {
    "cert": jwks,
    "iss": "xxx",
})
```
```live:jwt/verify/jwks/one_step:output
```

##### Using PEM encoded X.509 Certificate
The following examples will demonstrate verifying tokens using an X.509 Certificate
defined as:
```live:jwt/verify/cert:module
cert = `-----BEGIN CERTIFICATE-----
MIIBcDCCARagAwIBAgIJAMZmuGSIfvgzMAoGCCqGSM49BAMCMBMxETAPBgNVBAMM
CHdoYXRldmVyMB4XDTE4MDgxMDE0Mjg1NFoXDTE4MDkwOTE0Mjg1NFowEzERMA8G
A1UEAwwId2hhdGV2ZXIwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAATPwn3WCEXL
mjp/bFniDwuwsfu7bASlPae2PyWhqGeWwe23Xlyx+tSqxlkXYe4pZ23BkAAscpGj
yn5gXHExyDlKo1MwUTAdBgNVHQ4EFgQUElRjSoVgKjUqY5AXz2o74cLzzS8wHwYD
VR0jBBgwFoAUElRjSoVgKjUqY5AXz2o74cLzzS8wDwYDVR0TAQH/BAUwAwEB/zAK
BggqhkjOPQQDAgNIADBFAiEA4yQ/88ZrUX68c6kOe9G11u8NUaUzd8pLOtkKhniN
OHoCIHmNX37JOqTcTzGn2u9+c8NlnvZ0uDvsd1BmKPaUmjmm
-----END CERTIFICATE-----`
```
This example shows a two-step process to verify the token signature and then decode it for
further checks of the payload content. This approach gives more flexibility in verifying only
the claims that the policy needs to enforce.
```live:jwt/verify/cert/two_step:query:merge_down
io.jwt.verify_es256(es256_token, cert)                # Verify the token with the certificate
[header, payload, _] := io.jwt.decode(es256_token)    # Decode the token
payload.iss == "xxx"                                  # Ensure the issuer claim is the expected value
```
```live:jwt/verify/cert/two_step:output
```
The next example shows doing the same token signature verification, decoding, and content checks
but instead with a single call to `io.jwt.decode_verify`. Note that this gives less flexibility
in validating the payload content as **all** claims defined in the JWT spec are verified with the
provided constraints.
```live:jwt/verify/cert/one_step:query:merge_down
[valid, header, payload] := io.jwt.decode_verify(     # Decode and verify in one-step
    es256_token,
    {                                                 # With the supplied constraints:
        "cert": cert,                                 #   Verify the token with the certificate
        "iss": "xxx",                                 #   Ensure the issuer claim is the expected value
    }
)
```
```live:jwt/verify/cert/one_step:output
```

##### Round Trip - Sign and Verify
This example shows how to encode a token, verify, and decode it with the different options available.

Start with using the `io.jwt.encode_sign_raw` built-in:
```live:jwt/verify/round_trip_raw:module:hidden
```
```live:jwt/verify/round_trip_raw:query
raw_result_hs256 := io.jwt.encode_sign_raw(
    `{"alg":"HS256","typ":"JWT"}`,
    `{}`,
    `{"kty":"oct","k":"Zm9v"}`  	# "Zm9v" == base64url.encode_no_pad("foo")
)

# Important!! - Use the un-encoded plain text secret to verify and decode
raw_result_valid_hs256 := io.jwt.verify_hs256(raw_result_hs256, "foo")
raw_result_parts_hs256 := io.jwt.decode_verify(raw_result_hs256, {"secret": "foo"})
```
```live:jwt/verify/round_trip_raw:output
```

Now encode the and sign the same token contents but with `io.jwt.encode_sign` instead of the `raw` varient.
```live:jwt/verify/round_trip:module:hidden
```
```live:jwt/verify/round_trip:query:merge_down
result_hs256 := io.jwt.encode_sign(
    {
        "alg":"HS256",
        "typ":"JWT"
    },
    {},
    {
        "kty":"oct",
        "k":"Zm9v"
    }
)

# Important!! - Use the un-encoded plain text secret to verify and decode
result_parts_hs256 := io.jwt.decode_verify(result_hs256, {"secret": "foo"})
result_valid_hs256 := io.jwt.verify_hs256(result_hs256, "foo")
```
```live:jwt/verify/round_trip:output
```

> Note that the resulting encoded token is different from the first example using
> `io.jwt.encode_sign_raw`. The reason is that the `io.jwt.encode_sign` function
> is using canonicalized formatting for the header and payload whereas
> `io.jwt.encode_sign_raw` does not change the whitespace of the strings passed
> in. The decoded and parsed JSON values are still the same.

### Time

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := time.now_ns()``</span> | ``output`` is a ``number`` representing the current time since epoch in nanoseconds. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := time.parse_ns(layout, value)``</span> | ``output`` is a ``number`` representing the time ``value`` in nanoseconds since epoch. See the [Go `time` package documentation](https://golang.org/pkg/time/#Parse) for more details on ``layout``. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := time.parse_rfc3339_ns(value)``</span> | ``output`` is a ``number`` representing the time ``value`` in nanoseconds since epoch. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := time.parse_duration_ns(duration)``</span> | ``output`` is a ``number`` representing the duration ``duration`` in nanoseconds. See the [Go `time` package documentation](https://golang.org/pkg/time/#ParseDuration) for more details on ``duration``. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := time.date(ns)``<br/>``output := time.date([ns, tz])``</span> | ``output`` is of the form ``[year, month, day]``, which includes the ``year``, ``month`` (0-12), and ``day`` (0-31) as ``number``s representing the date from the nanoseconds since epoch (``ns``) in the timezone (``tz``), if supplied, or as UTC.| ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := time.clock(ns)``<br/>``output := time.clock([ns, tz])``</span> | ``output`` is of the form ``[hour, minute, second]``, which outputs the ``hour``, ``minute`` (0-59), and ``second`` (0-59) as ``number``s representing the time of day for the nanoseconds since epoch (``ns``) in the timezone (``tz``), if supplied, or as UTC. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``day := time.weekday(ns)``<br/>``day := time.weekday([ns, tz])``</span> | outputs the ``day`` as ``string`` representing the day of the week for the nanoseconds since epoch (``ns``) in the timezone (``tz``), if supplied, or as UTC. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := time.add_date(ns, years, months, days)``</span> | ``output`` is a ``number`` representing the time since epoch in nanoseconds after adding the ``years``, ``months`` and ``days`` to ``ns``. See the [Go `time` package documentation](https://golang.org/pkg/time/#Time.AddDate) for more details on ``add_date``. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := time.diff(ns1, ns2)``<br/>``output := time.diff([ns1, tz1], [ns2, tz2])``</span> | ``output`` is of the form ``[year(s), month(s), day(s), hour(s), minute(s), second(s)]``, which outputs ``year(s)``, ``month(s)`` (0-11), ``day(s)`` (0-30),  ``hour(s)``(0-23), ``minute(s)``(0-59) and ``second(s)``(0-59)  as ``number``s representing the  difference between the the two timestamps in nanoseconds since epoch (``ns1`` and ``ns2``), in the timezones (``tz1`` and ``tz2``, respectively), if supplied, or as UTC. | ``SDK-dependent`` |

> Multiple calls to the `time.now_ns` built-in function within a single policy
evaluation query will always return the same value.

Timezones can be specified as

* an [IANA Time Zone](https://www.iana.org/time-zones) string e.g. "America/New_York"
* "UTC" or "", which are equivalent to not passing a timezone (i.e. will return as UTC)
* "Local", which will use the local timezone.

Note that the opa executable will need access to the timezone files in the environment it is running in (see the [Go time.LoadLocation()](https://golang.org/pkg/time/#LoadLocation) documentation for more information).

### Cryptography

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := crypto.x509.parse_certificates(certs)``</span> | ``certs`` is base64 encoded DER or PEM data containing one or more certificates or a PEM string of one or more certificates. ``output`` is an array of X.509 certificates represented as JSON objects. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := crypto.x509.parse_certificate_request(csr)``</span> | ``csr`` is a base64 string containing either a PEM encoded or DER CSR or a string containing a PEM CSR.``output`` is an X.509 CSR represented as a JSON object. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := crypto.md5(string)``</span> | ``output`` is ``string`` md5 hashed. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := crypto.sha1(string)``</span> | ``output`` is ``string`` sha1 hashed. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := crypto.sha256(string)``</span> | ``output`` is ``string`` sha256 hashed. | ``SDK-dependent`` |

### Graphs

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``walk(x, [path, value])``</span> | ``walk`` is a relation that produces ``path`` and ``value`` pairs for documents under ``x``. ``path`` is ``array`` representing a pointer to ``value`` in ``x``.  Queries can use ``walk`` to traverse documents nested under ``x`` (recursively). | ✅ |
| <span class="opa-keep-it-together">``output := graph.reachable(graph, initial)``</span> | ``output`` is the set of vertices [reachable](https://en.wikipedia.org/wiki/Reachability) from the ``initial`` vertices in the directed ``graph``.  ``initial`` is a set or array of vertices, and ``graph`` is an object containing a set or array of neighboring vertices. | ✅ |

A common class of recursive rules can be reduced to a graph reachability
problem, so `graph.reachable` is useful for more than just graph analysis.
This usually requires some pre- and postprocessing.  The following example
shows you how to "flatten" a hierarchy of access permissions.

```live:graph/reachable/example:module
package graph_reachable_example

org_chart_data = {
  "ceo": {},
  "human_resources": {"owner": "ceo", "access": ["salaries", "complaints"]},
  "staffing": {"owner": "human_resources", "access": ["interviews"]},
  "internships": {"owner": "staffing", "access": ["blog"]}
}

org_chart_graph[entity_name] = edges {
  org_chart_data[entity_name]
  edges := {neighbor | org_chart_data[neighbor].owner == entity_name}
}

org_chart_permissions[entity_name] = access {
  org_chart_data[entity_name]
  reachable := graph.reachable(org_chart_graph, {entity_name})
  access := {item | reachable[k]; item := org_chart_data[k].access[_]}
}
```
```live:graph/reachable/example:query
org_chart_permissions[entity_name]
```
```live:graph/reachable/example:output
```

### HTTP

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``response := http.send(request)``</span> | ``http.send`` executes an HTTP `request` and returns a `response`. | ``SDK-dependent`` |

The `request` object parameter may contain the following fields:

| Field | Required | Type | Description |
| --- | --- | --- | --- |
| `url` | yes | `string` | HTTP URL to specify in the request (e.g., `"https://www.openpolicyagent.org"`). |
| `method` | yes | `string` | HTTP method to specify in request (e.g., `"GET"`, `"POST"`, `"PUT"`, etc.) |
| `body` | no | `any` | HTTP message body to include in request. The value will be serialized to JSON. |
| `raw_body` | no | `string` | HTTP message body to include in request. The value WILL NOT be serialized. Use this for non-JSON messages. |
| `headers` | no | `object` | HTTP headers to include in the request (e.g,. `{"X-Opa": "rules"}`). |
| `enable_redirect` | no | `boolean` | Follow HTTP redirects. Default: `false`. |
| `force_json_decode` | no | `boolean` | Decode the HTTP response message body as JSON even if the `Content-Type` header is missing. Default: `false`. |
| `tls_use_system_certs` | no | `boolean` | Use the system certificate pool. Default: `true` when `tls_ca_cert`, `tls_ca_cert_file`, `tls_ca_cert_env_variable` are unset. |
| `tls_ca_cert` | no | `string` | String containing a root certificate in PEM encoded format. |
| `tls_ca_cert_file` | no | `string` | Path to file containing a root certificate in PEM encoded format. |
| `tls_ca_cert_env_variable` | no | `string` | Environment variable containing a root certificate in PEM encoded format. |
| `tls_client_cert` | no | `string` | String containing a client certificate in PEM encoded format. |
| `tls_client_cert_file` | no | `string` | Path to file containing a client certificate in PEM encoded format. |
| `tls_client_cert_env_variable` | no | `string` | Environment variable containing a client certificate in PEM encoded format. |
| `tls_client_key` | no | `string` | String containing a key in PEM encoded format. |
| `tls_client_key_file` | no | `string` | Path to file containing a key in PEM encoded format. |
| `tls_client_key_env_variable` | no | `string` | Environment variable containing a client key in PEM encoded format. |
| `timeout` | no | `string` or `number` | Timeout for the HTTP request with a default of 5 seconds (`5s`). Numbers provided are in nanoseconds. Strings must be a valid duration string where a duration string is a possibly signed sequence of decimal numbers, each with optional fraction and a unit suffix, such as "300ms", "-1.5h" or "2h45m". Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h". A zero timeout means no timeout.|
| `tls_insecure_skip_verify` | no | `bool` | Allows for skipping TLS verification when calling a network endpoint. Not recommended for production. |
| `tls_server_name` | no | `string` | Sets the hostname that is sent in the client Server Name Indication and that be will be used for server certificate validation. If this is not set, the value of the `Host` header (if present) will be used. If neither are set, the host name from the requested URL is used. |
| `cache` | no | `boolean` | Cache HTTP response across OPA queries. Default: `false`. |
| `force_cache` | no | `boolean` | Cache HTTP response across OPA queries and override cache directives defined by the server. Default: `false`. |
| `force_cache_duration_seconds` | no | `number` | If `force_cache` is set, this field specifies the duration in seconds for the freshness of a cached response. |
| `raise_error` | no | `bool` | If `raise_error` is set, errors returned by `http.send` will halt policy evaluation. Default: `true`. |

If the `Host` header is included in `headers`, its value will be used as the `Host` header of the request. The `url` parameter will continue to specify the server to connect to.

When sending HTTPS requests with client certificates at least one the following combinations must be included

 * ``tls_client_cert`` and ``tls_client_key``
 * ``tls_client_cert_file`` and ``tls_client_key_file``
 * ``tls_client_cert_env_variable`` and ``tls_client_key_env_variable``

> To validate TLS server certificates, the user must also provide trusted root CA certificates through the ``tls_ca_cert``, ``tls_ca_cert_file`` and ``tls_ca_cert_env_variable`` fields. If the ``tls_use_system_certs`` field is ``true``, the system certificate pool will be used as well as any additional CA certificates.

The `response` object parameter will contain the following fields:

| Field | Type | Description |
| --- | --- | --- |
| `status` | `string` | HTTP status message (e.g., `"200 OK"`). |
| `status_code` | `number` | HTTP status code (e.g., `200`). If `raise_error` is `false`, this field will be set to `0` if `http.send` encounters an error. |
| `body` | `any` | Any JSON value. If the HTTP response message body was not deserialized from JSON, this field is set to `null`. |
| `raw_body` | `string` | The entire raw HTTP response message body represented as a string. |
| `headers` | `object` | An object containing the response headers. The values will be an array of strings, repeated headers are grouped under the same keys with all values in the array. |
| `error` | `object` | If `raise_error` is `false`, this field will represent the error encountered while running `http.send`. The `error` object contains a `message` key which holds the actual error message and a `code` key which represents if the error was caused due to a network issue or during policy evaluation. |

By default, an error returned by `http.send` halts the policy evaluation. This behaviour can be altered such that
instead of halting evaluation, if `http.send` encounters an error, it can return a `response` object with `status_code`
set to `0` and `error` describing the actual error. This can be activated by setting the `raise_error` field
in the `request` object to `false`.

If the `cache` field in the `request` object is `true`, `http.send` will return a cached response after it checks its freshness and validity.

`http.send` uses the `Cache-Control` and `Expires` response headers to check the freshness of the cached response.
Specifically if the [max-age](https://tools.ietf.org/html/rfc7234#section-5.2.2.8) `Cache-Control` directive is set, `http.send`
will use it to determine if the cached response is fresh or not. If `max-age` is not set, the `Expires` header will be used instead.

If the cached response is stale, `http.send` uses the `Etag` and `Last-Modified` response headers to check with the server if the
cached response is in fact still fresh. If the server responds with a `200` (`OK`) response, `http.send` will update the cache
with the new response. On a `304` (`Not Modified`) server response, `http.send` will update the headers in cached response with
their corresponding values in the `304` response.

The `force_cache` field can be used to override the cache directives defined by the server. This field is used in
conjunction with the `force_cache_duration_seconds` field. If `force_cache` is `true`, then `force_cache_duration_seconds`
**must** be specified and `http.send` will use this value to check the freshness of the cached response.

Also, if `force_cache` is `true`, it overrides the `cache` field.

> `http.send` uses the `Date` response header to calculate the current age of the response by comparing it with the current time.
> This value is used to determine the freshness of the cached response. As per https://tools.ietf.org/html/rfc7231#section-7.1.1.2,
> an origin server MUST NOT send a `Date` header field if it does not have a clock capable of providing a reasonable
> approximation of the current instance in Coordinated Universal Time. Hence, if `http.send` encounters a scenario where current
> age of the response is represented as a negative duration, the cached response will be considered as stale.

The table below shows examples of calling `http.send`:

| Example |  Comments |
| -------- |-----------|
| Accessing Google using System Cert Pool | ``http.send({"method": "get", "url": "https://www.google.com", "tls_use_system_certs": true })`` |
| Files containing TLS material | ``http.send({"method": "get", "url": "https://127.0.0.1:65331", "tls_ca_cert_file": "testdata/ca.pem", "tls_client_cert_file": "testdata/client-cert.pem", "tls_client_key_file": "testdata/client-key.pem"})`` |
| Environment variables containing TLS material | ``http.send({"method": "get", "url": "https://127.0.0.1:65360", "tls_ca_cert_env_variable": "CLIENT_CA_ENV", "tls_client_cert_env_variable": "CLIENT_CERT_ENV", "tls_client_key_env_variable": "CLIENT_KEY_ENV"})`` |

### Net

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``net.cidr_contains(cidr, cidr_or_ip)``</span> | `output` is `true` if `cidr_or_ip` (e.g. `127.0.0.64/26` or `127.0.0.1`) is contained within `cidr` (e.g. `127.0.0.1/24`) and false otherwise. Supports both IPv4 and IPv6 notations.| ✅ |
| <span class="opa-keep-it-together">``output := net.cidr_contains_matches(cidrs, cidrs_or_ips)``</span> | `output` is a `set` of tuples identifying matches where `cidrs_or_ips` are contained within `cidrs`. This function is similar to `net.cidr_contains` except it allows callers to pass collections of CIDRs or IPs as arguments and returns the matches (as opposed to a boolean result indicating a match between two CIDRs/IPs.) See below for examples. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``net.cidr_intersects(cidr1, cidr2)``</span> | `output` is `true` if `cidr1` (e.g. `192.168.0.0/16`) overlaps with `cidr2` (e.g. `192.168.1.0/24`) and false otherwise. Supports both IPv4 and IPv6 notations.| ✅ |
| <span class="opa-keep-it-together">``net.cidr_expand(cidr)``</span> | `output` is the set of hosts in `cidr`  (e.g., `net.cidr_expand("192.168.0.0/30")` generates 4 hosts: `{"192.168.0.0", "192.168.0.1", "192.168.0.2", "192.168.0.3"}` | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``net.cidr_merge(cidrs_or_ips)``</span> | `output` is the smallest possible set of CIDRs obtained after merging the provided list of IP addresses and subnets in `cidrs_or_ips`  (e.g., `net.cidr_merge(["192.0.128.0/24", "192.0.129.0/24"])` generates `{"192.0.128.0/23"}`. This function merges adjacent subnets where possible, those contained within others and also removes any duplicates. Supports both IPv4 and IPv6 notations. | ``SDK-dependent`` |

**`net.cidr_contains_matches` examples**

The `output := net.cidr_contains_matches(a, b)` function allows callers to supply
strings, arrays, sets, or objects for either `a` or `b`. The `output` value in
all cases is a set of tuples (2-element arrays) that identify matches, i.e.,
elements of `b` contained by elements of `a`. The first tuple element refers to
the match in `a` and the second tuple element refers to the match in `b`.

| Input Type | Output Type |
| --- | --- |
| `string` | `string` |
| `array` | `array` index |
| `set` | `set` element |
| `object` | `object` key |


```live:netcidrcontainsmatches:module:hidden
package netcidrcontainsmatches
```

If both operands are string values the function is similar to `net.cidr_contains`.

```live:netcidrcontainsmatches/strings:query:merge_down
net.cidr_contains_matches("1.1.1.0/24", "1.1.1.128")
```
```live:netcidrcontainsmatches/strings:output
```

Either (or both) operand(s) may be an array, set, or object.

```live:netcidrcontainsmatches/array:query:merge_down
net.cidr_contains_matches(["1.1.1.0/24", "1.1.2.0/24"], "1.1.1.128")
```
```live:netcidrcontainsmatches/array:output
```

The array/set/object elements may be arrays. In that case, the first element must be a valid CIDR/IP.

```live:netcidrcontainsmatches/tuples:query:merge_down
net.cidr_contains_matches([["1.1.0.0/16", "foo"], "1.1.2.0/24"], ["1.1.1.128", ["1.1.254.254", "bar"]])
```
```live:netcidrcontainsmatches/tuples:output
```

If the operand is a set, the outputs are matching elements. If the operand is an object, the outputs are matching keys.

```live:netcidrcontainsmatches/sets_and_objects:query:merge_down
net.cidr_contains_matches({["1.1.0.0/16", "foo"], "1.1.2.0/24"}, {"x": "1.1.1.128", "y": ["1.1.254.254", "bar"]})
```
```live:netcidrcontainsmatches/sets_and_objects:output:merge_down
```

### UUID

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := uuid.rfc4122(str)``</span> | ``output`` is ``string`` representing a version 4 uuid. For any given str the output will be consistent throughout a query evaluation. | ``SDK-dependent`` |

### Semantic Versions

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := semver.is_valid(str)``</span> | ``output`` is a ``boolean``. ``true`` means the input is a valid SemVer string (e.g. "1.0.0"). ``false`` is returned for invalid version strings and non-string input. | ``SDK-dependent`` |
| <span class="opa-keep-it-together">``output := semver.compare(str, str)``</span> | ``output`` is a ``number``. ``-1`` means the version in the first operand is less than the second. ``1`` means the version in the first operand is greater than the second. ``0`` means the versions are equal. Only valid SemVer strings are accepted e.g. ``1.2.3`` or ``0.1.0`` | ``SDK-dependent`` |

### Rego

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := rego.parse_module(filename, string)``</span> | ``rego.parse_module`` parses the input ``string`` as a Rego module and returns the AST as a JSON object ``output``. | ``SDK-dependent`` |

### OPA

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``output := opa.runtime()``</span> | ``opa.runtime`` returns a JSON object ``output`` that describes the runtime environment where OPA is deployed. **Caution**: Policies that depend on the output of ``opa.runtime`` may return different answers depending on how OPA was started. If possible, prefer using an explicit `input` or `data` value instead of `opa.runtime`. The ``output`` of ``opa.runtime`` will include a ``"config"`` key if OPA was started with a configuration file. The ``output`` of ``opa.runtime`` will include a ``"env"`` key containing the environment variables that the OPA process was started with. The ``output`` of ``opa.runtime`` will include ``"version"`` and ``"commit"`` keys containing the semantic version and build commit of OPA. | ``SDK-dependent`` |

### Debugging

| Built-in | Description | Wasm Support |
| ------- |-------------|---------------|
| <span class="opa-keep-it-together">``trace(string)``</span> | ``trace`` outputs the debug message ``string`` as a ``Note`` event in the query explanation. For example, ``trace("Hello There!")`` includes ``Note "Hello There!"`` in the query explanation. To print variables, use sprintf. For example, ``person := "Bob"; trace(sprintf("Hello There! %v", [person]))`` will emit ``Note "Hello There! Bob"``. | ``SDK-dependent`` |

## Reserved Names

The following words are reserved and cannot be used as variable names, rule
names, or dot-access style reference arguments:

```
as
default
else
false
import
package
not
null
some
true
with
```

## Grammar

Rego’s syntax is defined by the following grammar:

```ebnf
module          = package { import } policy
package         = "package" ref
import          = "import" package [ "as" var ]
policy          = { rule }
rule            = [ "default" ] rule-head { rule-body }
rule-head       = var [ "(" rule-args ")" ] [ "[" term "]" ] [ ( ":=" | "=" ) term ]
rule-args       = term { "," term }
rule-body       = [ "else" [ "=" term ] ] "{" query "}"
query           = literal { ( ";" | ( [CR] LF ) ) literal }
literal         = ( some-decl | expr | "not" expr ) { with-modifier }
with-modifier   = "with" term "as" term
some-decl       = "some" var { "," var }
expr            = term | expr-call | expr-infix
expr-call       = var [ "." var ] "(" [ term { "," term } ] ")"
expr-infix      = [ term "=" ] term infix-operator term
term            = ref | var | scalar | array | object | set | array-compr | object-compr | set-compr
array-compr     = "[" term "|" rule-body "]"
set-compr       = "{" term "|" rule-body "}"
object-compr    = "{" object-item "|" rule-body "}"
infix-operator  = bool-operator | arith-operator | bin-operator
bool-operator   = "==" | "!=" | "<" | ">" | ">=" | "<="
arith-operator  = "+" | "-" | "*" | "/"
bin-operator    = "&" | "|"
ref             = ( var | array | object | set | array-compr | object-compr | set-compr | expr-call ) { ref-arg }
ref-arg         = ref-arg-dot | ref-arg-brack
ref-arg-brack   = "[" ( scalar | var | array | object | set | "_" ) "]"
ref-arg-dot     = "." var
var             = ( ALPHA | "_" ) { ALPHA | DIGIT | "_" }
scalar          = string | NUMBER | TRUE | FALSE | NULL
string          = STRING | raw-string
raw-string      = "`" { CHAR-"`" } "`"
array           = "[" term { "," term } "]"
object          = "{" object-item { "," object-item } "}"
object-item     = ( scalar | ref | var ) ":" term
set             = empty-set | non-empty-set
non-empty-set   = "{" term { "," term } "}"
empty-set       = "set(" ")"
```

The grammar defined above makes use of the following syntax. See [the Wikipedia page on EBNF](https://en.wikipedia.org/wiki/Extended_Backus–Naur_Form) for more details:

```
[]     optional (zero or one instances)
{}     repetition (zero or more instances)
|      alternation (one of the instances)
()     grouping (order of expansion)
STRING JSON string
NUMBER JSON number
TRUE   JSON true
FALSE  JSON false
NULL   JSON null
CHAR   Unicode character
ALPHA  ASCII characters A-Z and a-z
DIGIT  ASCII characters 0-9
CR     Carriage Return
LF     Line Feed
```
