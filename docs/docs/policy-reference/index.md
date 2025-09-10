---
title: Policy Reference
sidebar_label: Overview
sidebar_position: 1
---

import BuiltinLegacyRedirect from "@site/src/components/BuiltinLegacyRedirect";

<BuiltinLegacyRedirect/>

## Assignment and Equality

```rego
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

```rego
# lookup value at index 0
val := arr[0]

 # check if value at index 0 is "foo"
"foo" == arr[0]

# find all indices i that have value "foo"
"foo" == arr[i]

# lookup last value
val := arr[count(arr)-1]

# with keywords
some 0, val in arr   # lookup value at index 0
0, "foo" in arr      # check if value at index 0 is "foo"
some i, "foo" in arr # find all indices i that have value "foo"
```

### Objects

```rego
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

# with keywords
o := {"foo": false}
# check if value exists: the expression will be true
false in o
# check if value for key "foo" is false
"foo", false in o
```

### Sets

```rego
# check if "foo" belongs to the set
a_set["foo"]

# check if "foo" DOES NOT belong to the set
not a_set["foo"]

# check if the array ["a", "b", "c"] belongs to the set
a_set[["a", "b", "c"]]

# find all arrays of the form [x, "b", z] in the set
a_set[[x, "b", z]]

# with keywords
"foo" in a_set
not "foo" in a_set
some ["a", "b", "c"] in a_set
some [x, "b", z] in a_set
```

## Iteration

### Arrays

```rego
# iterate over indices i
arr[i]

# iterate over values
val := arr[_]

# iterate over index/value pairs
val := arr[i]

# with keywords
some val in arr    # iterate over values
some i, _ in arr   # iterate over indices
some i, val in arr # iterate over index/value pairs
```

### Objects

```rego
# iterate over keys
obj[key]

# iterate over values
val := obj[_]

# iterate over key/value pairs
val := obj[key]

# with keywords
some val in obj      # iterate over values
some key, _ in obj   # iterate over keys
some key, val in obj # key/value pairs
```

### Sets

```rego
# iterate over values
set[val]

# with keywords
some val in set
```

### Advanced

```rego
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

```rego
# assert no values in set match predicate
count({x | set[x]; f(x)}) == 0

# assert all values in set make function f true
count({x | set[x]; f(x)}) == count(set)

# assert no values in set make function f true (using negation and helper rule)
not any_match

# assert all values in set make function f true (using negation and helper rule)
not any_not_match
```

```rego
# with keywords
any_match if {
    some x in set
    f(x)
}

any_not_match if {
    some x in set
    not f(x)
}
```

## Rules

In the examples below `...` represents one or more conditions.

### Constants

```rego
a := {1, 2, 3}
b := {4, 5, 6}
c := a | b
```

### Conditionals (Boolean)

```rego
# p is true if ...
p := true { ... }

# OR
# with keywords
p if { ... }

# OR
p { ... }
```

### Conditionals

```rego
# with keywords
default a := 1
a := 5   if { ... }
a := 100 if { ... }
```

### Incremental

```rego
# a_set will contain values of x and values of y
a_set[x] { ... }
a_set[y] { ... }

# alternatively, with keywords
a_set contains x if { ... }
a_set contains y if { ... }

# a_map will contain key->value pairs x->y and w->z
a_map[x] := y if { ... }
a_map[w] := z if { ... }
```

### Ordered (Else)

```rego
# with keywords
default a := 1
a := 5 if { ... }
else := 10 if { ... }
```

### Functions (Boolean)

```rego
# with keywords
f(x, y) if {
    ...
}

# OR

f(x, y) := true if {
    ...
}
```

### Functions (Conditionals)

```rego
# with keywords
f(x) := "A" if { x >= 90 }
f(x) := "B" if { x >= 80; x < 90 }
f(x) := "C" if { x >= 70; x < 80 }
```

### Reference Heads

```rego
# with keywords
fruit.apple.seeds = 12 if input == "apple"             # complete document (single value rule)

fruit.pineapple.colors contains x if x := "yellow"     # multi-value rule

fruit.banana.phone[x] = "bananular" if x := "cellular" # single value rule
fruit.banana.phone.cellular = "bananular" if true      # equivalent single value rule

fruit.orange.color(x) = true if x == "orange"          # function
```

For reasons of backwards-compatibility, partial sets need to use `contains` in
their rule heads, i.e.

```rego
fruit.box contains "apples" if true
```

whereas

```rego
fruit.box[x] if { x := "apples" }
```

defines a _complete document rule_ `fruit.box.apples` with value `true`.
The same is the case of rules with brackets that don't contain dots, like

```rego
box[x] if { x := "apples" } # => {"box": {"apples": true }}
box2[x] { x := "apples" } # => {"box": ["apples"]}
```

For backwards-compatibility, rules _without_ if and without _dots_ will be interpreted
as defining partial sets, like `box2`.

## Tests

```rego
# define a rule that starts with test_
test_NAME { ... }

# override input.foo value using the 'with' keyword
data.foo.bar.deny with input.foo as {"bar": [1,2,3]}}
```

## Built-in Functions

Rego's built-in functions offer policy authors tools for common policy
operations like JWT validation, signature verification, among many others.
The reference documentation for these functions can be found under [Built-in Functions](./policy-reference/builtins).

## Reserved Names

The following words are reserved and cannot be used as variable names, rule
names, or dot-access style reference arguments:

```
as
contains
data
default
else
every
false
if
in
import
input
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
import          = "import" ref [ "as" var ]
policy          = { rule }
rule            = [ "default" ] rule-head { rule-body }
rule-head       = ( ref | var ) ( rule-head-set | rule-head-obj | rule-head-func | rule-head-comp )
rule-head-comp  = [ assign-operator term ] [ "if" ]
rule-head-obj   = "[" term "]" [ assign-operator term ] [ "if" ]
rule-head-func  = "(" rule-args ")" [ assign-operator term ] [ "if" ]
rule-head-set   = "contains" term [ "if" ] | "[" term "]"
rule-args       = term { "," term }
rule-body       = [ "else" [ assign-operator term ] [ "if" ] ] ( "{" query "}" ) | literal
query           = literal { ( ";" | ( [CR] LF ) ) literal }
literal         = ( some-decl | expr | "not" expr ) { with-modifier }
with-modifier   = "with" term "as" term
some-decl       = "some" term { "," term } { "in" expr }
expr            = term | expr-call | expr-infix | expr-every | expr-parens | unary-expr
expr-call       = var [ "." var ] "(" [ expr { "," expr } ] ")"
expr-infix      = expr infix-operator expr
expr-every      = "every" var { "," var } "in" ( term | expr-call | expr-infix ) "{" query "}"
expr-parens     = "(" expr ")"
unary-expr      = "-" expr
membership      = term [ "," term ] "in" term
term            = ref | var | scalar | array | object | set | membership | array-compr | object-compr | set-compr
array-compr     = "[" term "|" query "]"
set-compr       = "{" term "|" query "}"
object-compr    = "{" object-item "|" query "}"
infix-operator  = assign-operator | bool-operator | arith-operator | bin-operator
bool-operator   = "==" | "!=" | "<" | ">" | ">=" | "<="
arith-operator  = "+" | "-" | "*" | "/" | "%"
bin-operator    = "&" | "|"
assign-operator = ":=" | "="
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
