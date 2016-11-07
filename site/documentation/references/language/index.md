---
nav_id: MAIN_DOCUMENTATION
doc_id: LANGUAGE_REFERENCE
layout: documentation

title: Language Reference
---

{% contentfor header %}

# Language Reference

This document is the authoritative specification of the Rego policy language
(V1). All policies in OPA are written in Rego.

{% endcontentfor %}

{% contentfor body %}

## Built-in Functions

The built-in functions for the language provide basic operations to manipulate
scalar values (e.g. numbers and strings), and aggregate functions that summarize
complex types.

### Inequality

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| ``x != y``   |  2     | x is not equal to y |
| ``x < y``   |  2     | x is less than y |
| ``x <= y``   |  2     | x is less than or equal to y |
| ``x > y``   |  2     | x is greater than y |
| ``x >= y``   |  2     | x is greater than or equal to y |

### Numbers

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| ``plus(x, y, output)``   |  2     | x + y = output |
| ``minus(x, y, output)``  |  2     | x - y = output |
| ``mul(x, y, output)``   |  2     | x * y = output |
| ``div(x, y, output)``   |  2     | x / y = output |
| ``round(x, output)``    |  1     | output is x rounded to the nearest integer |
| ``abs(x, output)``    |  1     | output is the absolute value of x |

### Aggregates

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| ``count(collection, output)`` | 1 | output is the length of the object, array, or set |
| ``sum(array_or_set, output)`` | 1 | output is the sum of the numbers in array or set |
| ``max(array_or_set, output)`` | 1 | output is the maximum value in the array or set |

### Types

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| ``to_number(x, output)`` | 1 | output is x converted to a number |

### Strings

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``format_int(number, base, output)``</span> | 2 | output is string representation of number in given base |
| ``concat(join, array_or_set, output)`` | 2 | output is the result of concatenating the elements of array or set with the join string |
|``re_match(pattern, value)`` | 2 | true if the value matches the pattern |

## <a name="grammar"></a> Grammar

Rego’s syntax is defined by the following grammar:

```
module         = package { import } policy
package        = "package" ref
import         = "import" package [ "as" var ]
policy         = { rule }
rule           = rule-head [ ":-" rule-body ]
rule-head      = var [ "[" term "]" ] [ = term ]
rule-body      = [ literal { "," literal } ]
literal        = expr | "not" expr
expr           = term | expr-built-in | expr-infix
expr-built-in  = var "(" [ term { , term } ] ")"
expr-infix     = term bool-operator term
term           = ref | var | scalar | array | object | set | array-compr
array-compr    = "[" term "|" rule-body "]"
bool-operator  = "=" | "!=" | "<" | ">" | ">=" | "<="
ref            = var { ref-arg }
ref-arg        = ref-arg-dot | ref-arg-brack
ref-arg-brack  = "[" ( scalar | var | "_" ) "]"
ref-arg-dot    = "." var
var            = ( ALPHA | "_" ) { ALPHA | DIGIT | "_" }
scalar         = STRING | NUMBER | TRUE | FALSE | NULL
array          = "[" term { "," term } "]"
object         = "{" object-item { "," object-item } "}"
object-item    = ( scalar | ref | var ) ":" term
set            = empty-set | non-empty-set
non-empty-set  = "{" term { "," term } "}"
empty-set      = "set(" ")"
```
{: .opa-collapse--ignore}

The grammar defined above makes use of the following syntax. See [the Wikipedia page on EBNF](https://en.wikipedia.org/wiki/Extended_Backus–Naur_Form) for more details:

```
[]     optional (zero or one instances)
{}     repetition (zero or more instances)
|      alteration (one of the instances)
()     grouping (order of expansion)
STRING JSON string
NUMBER JSON number
TRUE   JSON true
FALSE  JSON false
NULL   JSON null
ALPHA  ASCII characters A-Z and a-z
DIGIT  ASCII characters 0-9
```
{: .opa-collapse--ignore}

{% endcontentfor %}
