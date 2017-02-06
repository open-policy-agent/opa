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
| <span class="opa-keep-it-together">``x != y``</span>   |  2     | ``x`` is not equal to ``y`` |
| <span class="opa-keep-it-together">``x < y``</span>   |  2     | ``x`` is less than ``y`` |
| <span class="opa-keep-it-together">``x <= y``</span>   |  2     | ``x`` is less than or equal to ``y`` |
| <span class="opa-keep-it-together">``x > y``</span>   |  2     | ``x`` is greater than ``y`` |
| <span class="opa-keep-it-together">``x >= y``</span>   |  2     | ``x`` is greater than or equal to ``y`` |

### Numbers

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``z = x + y``</span>   |  2     | ``z`` is the sum of ``x`` and ``y`` |
| <span class="opa-keep-it-together">``z = x - y``</span>  |  2     | ``z`` is the difference of ``x`` and ``y`` |
| <span class="opa-keep-it-together">``z = x * y``</span>   |  2     | ``z`` is the product of ``x`` and ``y`` |
| <span class="opa-keep-it-together">``z = x / y``</span>   |  2     | ``z`` is the quotient of ``x`` and ``y``  |
| <span class="opa-keep-it-together">``round(x, output)``</span>    |  1     | ``output`` is ``x`` rounded to the nearest integer |
| <span class="opa-keep-it-together">``abs(x, output)``</span>    |  1     | ``output`` is the absolute value of ``x`` |

### Aggregates

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``count(collection, output)``</span> | 1 | ``output`` is the length of the object, array, or set ``collection`` |
| <span class="opa-keep-it-together">``sum(array_or_set, output)``</span> | 1 | ``output`` is the sum of the numbers in ``array_or_set`` |
| <span class="opa-keep-it-together">``max(array_or_set, output)``</span> | 1 | ``output`` is the maximum value in ``array_or_set`` |

### Sets

| Built-in | Inputs | Description |
| -------- | ------ | ----------- |
| <span class="opa-keep-it-together">``s3 = s1 & s2``</span> | 2 | ``s3`` is the intersection of ``s1`` and ``s2``. |
| <span class="opa-keep-it-together">``s3 = s1 | s2``</span> | 2 | ``s3`` is the union of ``s1`` and ``s2``. |
| <span class="opa-keep-it-together">``s3 = s1 - s2``</span> | 2 | ``s3`` is the difference between ``s1`` and ``s2``, i.e., the elements in ``s1`` that are not in ``s2`` |

### Strings

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``concat(join, array_or_set, output)``</span> | 2 | ``output`` is the result of concatenating the elements of ``array_or_set`` with the  string ``join`` |
| <span class="opa-keep-it-together">``contains(string, search)``</span> | 2 | true if ``string`` contains ``search`` |
| <span class="opa-keep-it-together">``endswith(string, search)``</span> | 2 | true if ``string`` ends with ``search`` |
| <span class="opa-keep-it-together">``format_int(number, base, output)``</span> | 2 | ``output`` is string representation of ``number`` in the given ``base`` |
| <span class="opa-keep-it-together">``indexof(string, search, output)``</span> | 2 | ``output`` is the index inside ``string`` where ``search`` first occurs, or -1 if ``search`` does not exist |
| <span class="opa-keep-it-together">``lower(string, output)``</span> | 1 | ``output`` is ``string`` after converting to lower case |
| <span class="opa-keep-it-together">``re_match(pattern, value)``</span> | 2 | true if the value matches the pattern |
| <span class="opa-keep-it-together">``startswith(string, search)``</span> | 2 | true if ``string`` begins with ``search`` |
| <span class="opa-keep-it-together">``substring(string, start, length, output)``</span> | 2 | ``output`` is the portion of ``string`` from index ``start`` and having a length of ``length``.  If ``length`` is less than zero, ``length`` is the remainder of the ``string``. |
| <span class="opa-keep-it-together">``upper(string, output)``</span> | 1 | ``output`` is ``string`` after converting to upper case |

### Types

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``to_number(x, output)``</span> | 1 | ``output`` is ``x`` converted to a number |

## <a name="reserved"></a> Reserved Names

The following words are reserved and cannot be used as variable names, rule
names, or dot-access style reference arguments:

```
as
default
false
import
package
not
null
true
with
```

## <a name="grammar"></a> Grammar

Rego’s syntax is defined by the following grammar:

```
module         = package { import } policy
package        = "package" ref
import         = "import" package [ "as" var ]
policy         = { rule }
rule           = [ "default" ] rule-head [ ":-" rule-body ]
rule-head      = var [ "[" term "]" ] [ = term ]
rule-body      = [ literal { "," literal } ]
literal        = ( expr | "not" expr ) { with-modifier }
with-modifier  = "with" term "as" term
expr           = term | expr-built-in | expr-infix
expr-built-in  = var "(" [ term { , term } ] ")"
expr-infix     = [ term "=" ] term infix-operator term
term           = ref | var | scalar | array | object | set | array-compr
array-compr    = "[" term "|" rule-body "]"
infix-operator = bool-operator | arith-operator | bin-operator
bool-operator  = "=" | "!=" | "<" | ">" | ">=" | "<="
arith-operator = "+" | "-" | "*" | "/"
bin-operator   = "&" | "|"
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
