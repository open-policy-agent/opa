# Language Reference

This document is the authoritative specification of the Rego policy language
(V1). All policies in OPA are written in Rego.

## <a name="built-in-functions"/>Built-in Functions

The built-in functions for the language provide basic operations to manipulate
scalar values (e.g. numbers and strings), and aggregate functions that summarize
complex types.

### <a name="inequality"/>Inequality

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``x != y``</span>   |  2     | ``x`` is not equal to ``y`` |
| <span class="opa-keep-it-together">``x < y``</span>   |  2     | ``x`` is less than ``y`` |
| <span class="opa-keep-it-together">``x <= y``</span>   |  2     | ``x`` is less than or equal to ``y`` |
| <span class="opa-keep-it-together">``x > y``</span>   |  2     | ``x`` is greater than ``y`` |
| <span class="opa-keep-it-together">``x >= y``</span>   |  2     | ``x`` is greater than or equal to ``y`` |

### <a name="numbers"/>Numbers

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``z = x + y``</span>   |  2     | ``z`` is the sum of ``x`` and ``y`` |
| <span class="opa-keep-it-together">``z = x - y``</span>  |  2     | ``z`` is the difference of ``x`` and ``y`` |
| <span class="opa-keep-it-together">``z = x * y``</span>   |  2     | ``z`` is the product of ``x`` and ``y`` |
| <span class="opa-keep-it-together">``z = x / y``</span>   |  2     | ``z`` is the quotient of ``x`` and ``y``  |
| <span class="opa-keep-it-together">``round(x, output)``</span>    |  1     | ``output`` is ``x`` rounded to the nearest integer |
| <span class="opa-keep-it-together">``abs(x, output)``</span>    |  1     | ``output`` is the absolute value of ``x`` |

### <a name="aggregates"/>Aggregates

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``count(collection, output)``</span> | 1 | ``output`` is the length of the object, array, or set ``collection`` |
| <span class="opa-keep-it-together">``sum(array_or_set, output)``</span> | 1 | ``output`` is the sum of the numbers in ``array_or_set`` |
| <span class="opa-keep-it-together">``max(array_or_set, output)``</span> | 1 | ``output`` is the maximum value in ``array_or_set`` |
| <span class="opa-keep-it-together">``min(array_or_set, output)``</span> | 1 | ``output`` is the minimum value in ``array_or_set`` |

### <a name="sets"/>Sets

| Built-in | Inputs | Description |
| -------- | ------ | ----------- |
| <span class="opa-keep-it-together">``s3 = s1 & s2``</span> | 2 | ``s3`` is the intersection of ``s1`` and ``s2``. |
| <span class="opa-keep-it-together">``s3 = s1 | s2``</span> | 2 | ``s3`` is the union of ``s1`` and ``s2``. |
| <span class="opa-keep-it-together">``s3 = s1 - s2``</span> | 2 | ``s3`` is the difference between ``s1`` and ``s2``, i.e., the elements in ``s1`` that are not in ``s2`` |

### <a name="strings"/>Strings

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``concat(join, array_or_set, output)``</span> | 2 | ``output`` is the result of concatenating the elements of ``array_or_set`` with the  string ``join`` |
| <span class="opa-keep-it-together">``contains(string, search)``</span> | 2 | true if ``string`` contains ``search`` |
| <span class="opa-keep-it-together">``endswith(string, search)``</span> | 2 | true if ``string`` ends with ``search`` |
| <span class="opa-keep-it-together">``format_int(number, base, output)``</span> | 2 | ``output`` is string representation of ``number`` in the given ``base`` |
| <span class="opa-keep-it-together">``indexof(string, search, output)``</span> | 2 | ``output`` is the index inside ``string`` where ``search`` first occurs, or -1 if ``search`` does not exist |
| <span class="opa-keep-it-together">``lower(string, output)``</span> | 1 | ``output`` is ``string`` after converting to lower case |
| <span class="opa-keep-it-together">``re_match(pattern, value)``</span> | 2 | true if the value matches the pattern |
| <span class="opa-keep-it-together">``replace(string, old, new, output)``</span> | 3 | ``output`` is a ``string`` representing ``string`` with all instances of ``old`` replaced by ``new`` |
| <span class="opa-keep-it-together">``split(string, delimiter, output)``</span> | 2 | ``output`` is ``array[string]`` representing elements of ``string`` separated by ``delimiter`` |
| <span class="opa-keep-it-together">``sprintf(string, values, output)``</span> | 2 | ``output`` is a ``string`` representing ``string`` formatted by the values in the ``array`` ``values``. |
| <span class="opa-keep-it-together">``startswith(string, search)``</span> | 2 | true if ``string`` begins with ``search`` |
| <span class="opa-keep-it-together">``substring(string, start, length, output)``</span> | 2 | ``output`` is the portion of ``string`` from index ``start`` and having a length of ``length``.  If ``length`` is less than zero, ``length`` is the remainder of the ``string``. |
| <span class="opa-keep-it-together">``trim(string, cutset, output)``</span> | 2 | ``output`` is a ``string`` representing ``string`` with all leading and trailing instances of the characters in ``cutset`` removed. |
| <span class="opa-keep-it-together">``upper(string, output)``</span> | 1 | ``output`` is ``string`` after converting to upper case |

### <a name="types"/>Types

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``to_number(x, output)``</span> | 1 | ``output`` is ``x`` converted to a number |

### <a name="encoding"/>Encoding

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``base64url.encode(x, output)``</span> | 1 | ``output`` is ``x`` serialized to a base64url encoded string |
| <span class="opa-keep-it-together">``base64url.decode(string, output)``</span> | 1 | ``output`` is ``string`` deserialized from a base64url encoding string |
| <span class="opa-keep-it-together">``json.marshal(x, output)``</span> | 1 | ``output`` is ``x`` serialized to a JSON string |
| <span class="opa-keep-it-together">``json.unmarshal(string, output)``</span> | 1 | ``output`` is ``string`` deserialized to a term from a JSON encoded string |
| <span class="opa-keep-it-together">``yaml.marshal(x, output)``</span> | 1 | ``output`` is ``x`` serialized to a YAML string |
| <span class="opa-keep-it-together">``yaml.unmarshal(string, output)``</span> | 1 | ``output`` is ``string`` deserialized to a term from YAML encoded string |

### <a name="tokens"/>Tokens

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``io.jwt.decode(string, header, payload, sig)``</span> | 1 | ``header`` and ``payload`` are ``object``. ``signature`` is the hexadecimal representation of the signature on the token. |

The input `string` is a JSON Web Token encoded with JWS Compact Serialization. JWE and JWS JSON Serialization are not supported. If nested signing was used, the ``header``, ``payload`` and ``signature`` will represent the most deeply nested token.

### <a name="time"/>Time

| Built-in | Inputs | Description |
| -------- | ------ | ----------- |
| <span class="opa-keep-it-together">``time.now_ns(output)``</span> | 0 | ``output`` is ``number`` representing the current time since epoch in nanoseconds. |
| <span class="opa-keep-it-together">``time.parse_ns(layout, value, output)``</span> | 2 | ``output`` is ``number`` representing the time ``value`` in nanoseconds since epoch. See the [Go `time` package documentation](https://golang.org/pkg/time/#Parse) for more details on `layout`. `` |
| <span class="opa-keep-it-together">``time.parse_rfc3339_ns(value, output)``</span> | 1 | ``output`` is ``number`` representing the time ``value`` in nanoseconds since epoch.`` |

> Multiple calls to the `time.now_ns` built-in function within a single policy
evaluation query will always return the same value.

### <a name="graphs"/>Graphs

| Built-in | Inputs | Description |
| --- | --- | --- |
| <span class="opa-keep-it-together">``walk(x, [path, value])``</span> | 0 | ``path`` is ``array`` representing a pointer to ``value`` in ``x``.  Queries can use ``walk`` to traverse documents nested under ``x`` (recursively). |

## <a name="reserved-names"></a> Reserved Names

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
true
with
```

## <a name="grammar"></a> Grammar

Rego’s syntax is defined by the following grammar:

```
module          = package { import } policy
package         = "package" ref
import          = "import" package [ "as" var ]
policy          = { rule }
rule            = [ "default" ] rule-head { rule-body }
rule-head       = var [ "[" term "]" ] [ = term ]
rule-body       = [ else [ = term ] ] "{" query "}"
function        = func-head func-body
func-head       = var "(" [ arg-term { , arg-term } ] ")" = term
func-body       = "{" instructions "}"
query           = literal { ";" | [\r\n] literal }
literal         = ( expr | "not" expr ) { with-modifier }
with-modifier   = "with" term "as" term
instructions    = expr { ";" | [\r\n] expr }
expr            = term | expr-built-in | expr-infix
expr-built-in   = var [ "." var ] "(" [ term { , term } ] ")"
expr-infix      = [ term "=" ] term infix-operator term
term            = ref | var | scalar | array | object | set | array-compr
arg-term        = scalar | var | arg-object | arg-array
array-compr     = "[" term "|" rule-body "]"
set-compr       = "{" term "|" rule-body "}"
object-compr    = "{" object-item "|" rule-body "}"
infix-operator  = bool-operator | arith-operator | bin-operator
bool-operator   = "=" | "!=" | "<" | ">" | ">=" | "<="
arith-operator  = "+" | "-" | "*" | "/"
bin-operator    = "&" | "|"
ref             = var { ref-arg }
ref-arg         = ref-arg-dot | ref-arg-brack
ref-arg-brack   = "[" ( scalar | var | array | object | set | "_" ) "]"
ref-arg-dot     = "." var
var             = ( ALPHA | "_" ) { ALPHA | DIGIT | "_" }
scalar          = string | NUMBER | TRUE | FALSE | NULL
string          = STRING | raw-string
raw-string      = "`" { CHAR-"`" } "`"
array           = "[" term { "," term } "]"
arg-array       = "[" arg-term { "," arg-term } "]"
object          = "{" object-item { "," object-item } "}"
object-item     = ( scalar | ref | var ) ":" term
arg-object      = "{" arg-object-item { "," arg-object-item } "}"
arg-object-item = ( scalar | ref ) ":" arg-term
set             = empty-set | non-empty-set
non-empty-set   = "{" term { "," term } "}"
empty-set       = "set(" ")"
```

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
CHAR   Unicode character
ALPHA  ASCII characters A-Z and a-z
DIGIT  ASCII characters 0-9
```
