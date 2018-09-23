# Language Reference

This document is the authoritative specification of the Rego policy language
(V1). All policies in OPA are written in Rego.

## Built-in Functions

The built-in functions for the language provide basic operations to manipulate
scalar values (e.g. numbers and strings), and aggregate functions that summarize
complex types.

### Comparison

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``x == y``</span>   |  2     | ``x`` is equal to ``y`` |
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
| <span class="opa-keep-it-together">``z = x % y``</span>   |  2     | ``z`` is the remainder from the division of ``x`` and ``y``  |
| <span class="opa-keep-it-together">``round(x, output)``</span>    |  1     | ``output`` is ``x`` rounded to the nearest integer |
| <span class="opa-keep-it-together">``abs(x, output)``</span>    |  1     | ``output`` is the absolute value of ``x`` |

### Aggregates

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``count(collection, output)``</span> | 1 | ``output`` is the length of the object, array, or set ``collection`` |
| <span class="opa-keep-it-together">``sum(array_or_set, output)``</span> | 1 | ``output`` is the sum of the numbers in ``array_or_set`` |
| <span class="opa-keep-it-together">``product(array_or_set, output)``</span> | 1 | ``output`` is the product of the numbers in ``array_or_set`` |
| <span class="opa-keep-it-together">``max(array_or_set, output)``</span> | 1 | ``output`` is the maximum value in ``array_or_set`` |
| <span class="opa-keep-it-together">``min(array_or_set, output)``</span> | 1 | ``output`` is the minimum value in ``array_or_set`` |
| <span class="opa-keep-it-together">``sort(array_or_set, output)``</span> | 1 | ``output`` is the sorted ``array`` containing elements from ``array_or_set``. |
| <span class="opa-keep-it-together">``all(array_or_set, output)``</span> | 1 | ``output`` is ``true`` if all of the values in ``array_or_set`` are ``true``. A collection of length 0 returns ``true``.|
| <span class="opa-keep-it-together">``any(array_or_set, output)``</span> | 1 | ``output`` is ``true`` if any of the values in ``array_or_set`` is ``true``. A collection of length 0 returns ``false``.|

### Arrays

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``array.concat(array, array, output)``</span> | 2 | ``output`` is the result of concatenating the two input arrays together. |

### Sets

| Built-in | Inputs | Description |
| -------- | ------ | ----------- |
| <span class="opa-keep-it-together">``s3 = s1 & s2``</span> | 2 | ``s3`` is the intersection of ``s1`` and ``s2``. |
| <span class="opa-keep-it-together"><code>s3 = s1 &#124; s2</code></span> | 2 | ``s3`` is the union of ``s1`` and ``s2``. |
| <span class="opa-keep-it-together">``s3 = s1 - s2``</span> | 2 | ``s3`` is the difference between ``s1`` and ``s2``, i.e., the elements in ``s1`` that are not in ``s2`` |
| <span class="opa-keep-it-together">``intersection(set[set], output)``</span> | 1 | ``output`` is the intersection of the sets in the input set  |
| <span class="opa-keep-it-together">``union(set[set], output)``</span> | 1 | ``output`` is the union of the sets in the input set  |

### Strings

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``concat(delimiter, array_or_set, output)``</span> | 2 | ``output`` is the result of joining together the elements of ``array_or_set`` with the  string ``delimiter`` |
| <span class="opa-keep-it-together">``contains(string, search)``</span> | 2 | true if ``string`` contains ``search`` |
| <span class="opa-keep-it-together">``endswith(string, search)``</span> | 2 | true if ``string`` ends with ``search`` |
| <span class="opa-keep-it-together">``format_int(number, base, output)``</span> | 2 | ``output`` is string representation of ``number`` in the given ``base`` |
| <span class="opa-keep-it-together">``indexof(string, search, output)``</span> | 2 | ``output`` is the index inside ``string`` where ``search`` first occurs, or -1 if ``search`` does not exist |
| <span class="opa-keep-it-together">``lower(string, output)``</span> | 1 | ``output`` is ``string`` after converting to lower case |
| <span class="opa-keep-it-together">``replace(string, old, new, output)``</span> | 3 | ``output`` is a ``string`` representing ``string`` with all instances of ``old`` replaced by ``new`` |
| <span class="opa-keep-it-together">``split(string, delimiter, output)``</span> | 2 | ``output`` is ``array[string]`` representing elements of ``string`` separated by ``delimiter`` |
| <span class="opa-keep-it-together">``sprintf(string, values, output)``</span> | 2 | ``output`` is a ``string`` representing ``string`` formatted by the values in the ``array`` ``values``. |
| <span class="opa-keep-it-together">``startswith(string, search)``</span> | 2 | true if ``string`` begins with ``search`` |
| <span class="opa-keep-it-together">``substring(string, start, length, output)``</span> | 2 | ``output`` is the portion of ``string`` from index ``start`` and having a length of ``length``.  If ``length`` is less than zero, ``length`` is the remainder of the ``string``. |
| <span class="opa-keep-it-together">``trim(string, cutset, output)``</span> | 2 | ``output`` is a ``string`` representing ``string`` with all leading and trailing instances of the characters in ``cutset`` removed. |
| <span class="opa-keep-it-together">``upper(string, output)``</span> | 1 | ``output`` is ``string`` after converting to upper case |

### Regex
| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``re_match(pattern, value)``</span> | 2 | true if the ``value`` matches the regex ``pattern`` |
| <span class="opa-keep-it-together">``regex.split(pattern, string, output)``</span> | 2 | ``output`` is ``array[string]`` representing elements of ``string`` separated by ``pattern`` |
| <span class="opa-keep-it-together">``regex.globs_match(glob1, glob2)``</span> | 2 | true if the intersection of regex-style globs ``glob1`` and ``glob2`` matches a non-empty set of non-empty strings. The set of regex symbols is limited for this builtin: only ``.``, ``*``, ``+``, ``[``, ``-``, ``]`` and ``\`` are treated as special symbols. |
| <span class="opa-keep-it-together">``regex.template_match(patter, string, delimiter_start, delimiter_end, output)``</span> | 4 | ``output`` is true if ``string`` matches ``pattern``. ``pattern`` is a string containing ``0..n`` regular expressions delimited by ``delimiter_start`` and ``delimiter_end``. Example ``regex.template_match("urn:foo:{.*}", "urn:foo:bar:baz", "{", "}", x)`` returns ``true`` for ``x``. |

### Types

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``to_number(x, output)``</span> | 1 | ``output`` is ``x`` converted to a number |
| <span class="opa-keep-it-together">``is_number(x, output)``</span> | 1 | ``output`` is ``true`` if ``x`` is a number |
| <span class="opa-keep-it-together">``is_string(x, output)``</span> | 1 | ``output`` is ``true`` if ``x`` is a string |
| <span class="opa-keep-it-together">``cast_string(x, output)``</span> | 1 | ``output`` is ``x`` cast to a string |
| <span class="opa-keep-it-together">``is_boolean(x, output)``</span> | 1 | ``output`` is ``true`` if ``x`` is a boolean |
| <span class="opa-keep-it-together">``cast_boolean(x, output)``</span> | 1 | ``output`` is ``x`` cast to a boolean |
| <span class="opa-keep-it-together">``is_array(x, output)``</span> | 1 | ``output`` is ``true`` if ``x`` is an array |
| <span class="opa-keep-it-together">``cast_array(x, output)``</span> | 1 | ``output`` is ``x`` cast to an array |
| <span class="opa-keep-it-together">``is_set(x, output)``</span> | 1 | ``output`` is ``true`` if ``x`` is a set |
| <span class="opa-keep-it-together">``cast_set(x, output)``</span> | 1 | ``output`` is ``x`` cast to a set |
| <span class="opa-keep-it-together">``is_object(x, output)``</span> | 1 | ``output`` is ``true`` if ``x`` is an object |
| <span class="opa-keep-it-together">``cast_object(x, output)``</span> | 1 | ``output`` is ``x`` cast to an object |
| <span class="opa-keep-it-together">``is_null(x, output)``</span> | 1 | ``output`` is ``true`` if ``x`` is null |
| <span class="opa-keep-it-together">``cast_null(x, output)``</span> | 1 | ``output`` is ``x`` cast to null |
| <span class="opa-keep-it-together">``type_name(x, output)``</span> | 1 | ``output`` is the type of ``x`` |

### Encoding

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``base64.encode(x, output)``</span> | 1 | ``output`` is ``x`` serialized to a base64 encoded string |
| <span class="opa-keep-it-together">``base64.decode(string, output)``</span> | 1 | ``output`` is ``x`` deserialized from a base64 encoding string |
| <span class="opa-keep-it-together">``base64url.encode(x, output)``</span> | 1 | ``output`` is ``x`` serialized to a base64url encoded string |
| <span class="opa-keep-it-together">``base64url.decode(string, output)``</span> | 1 | ``output`` is ``string`` deserialized from a base64url encoding string |
| <span class="opa-keep-it-together">``urlquery.encode(string, output)``</span> | 1 | ``output`` is ``string`` serialized to a URL query parameter encoded string |
| <span class="opa-keep-it-together">``urlquery.encode_object(object, output)``</span> | 1 | ``output`` is ``object`` serialized to a URL query parameter encoded string |
| <span class="opa-keep-it-together">``urlquery.decode(string, output)``</span> | 1 | ``output`` is ``string`` deserialized from a URL query parameter encoded string |
| <span class="opa-keep-it-together">``json.marshal(x, output)``</span> | 1 | ``output`` is ``x`` serialized to a JSON string |
| <span class="opa-keep-it-together">``json.unmarshal(string, output)``</span> | 1 | ``output`` is ``string`` deserialized to a term from a JSON encoded string |
| <span class="opa-keep-it-together">``yaml.marshal(x, output)``</span> | 1 | ``output`` is ``x`` serialized to a YAML string |
| <span class="opa-keep-it-together">``yaml.unmarshal(string, output)``</span> | 1 | ``output`` is ``string`` deserialized to a term from YAML encoded string |

### Tokens

| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``io.jwt.verify_rs256(string, certificate, output)``</span> | 1 | ``output`` is ``true`` if the RS256 signature of the input token is valid. ``certificate`` is the PEM encoded certificate used to verify the RS256 signature|
| <span class="opa-keep-it-together">``io.jwt.verify_ps256(string, certificate, output)``</span> | 1 | ``output`` is ``true`` if the PS256 signature of the input token is valid. ``certificate`` is the PEM encoded certificate used to verify the PS256 signature|
| <span class="opa-keep-it-together">``io.jwt.verify_es256(string, certificate, output)``</span> | 1 | ``output`` is ``true`` if the ES256 signature of the input token is valid. ``certificate`` is the PEM encoded certificate used to verify the ES256 signature|
| <span class="opa-keep-it-together">``io.jwt.verify_hs256(string, secret, output)``</span> | 1 | ``output`` is ``true`` if the Secret signature of the input token is valid. ``secret`` is a plain text secret used to verify the HS256 signature|
| <span class="opa-keep-it-together">``io.jwt.decode(string, [header, payload, sig])``</span> | 1 | ``header`` and ``payload`` are ``object``. ``signature`` is the hexadecimal representation of the signature on the token. |

The input `string` is a JSON Web Token encoded with JWS Compact Serialization. JWE and JWS JSON Serialization are not supported. If nested signing was used, the ``header``, ``payload`` and ``signature`` will represent the most deeply nested token.

### Time

| Built-in | Inputs | Description |
| -------- | ------ | ----------- |
| <span class="opa-keep-it-together">``time.now_ns(output)``</span> | 0 | ``output`` is ``number`` representing the current time since epoch in nanoseconds. |
| <span class="opa-keep-it-together">``time.parse_ns(layout, value, output)``</span> | 2 | ``output`` is ``number`` representing the time ``value`` in nanoseconds since epoch. See the [Go `time` package documentation](https://golang.org/pkg/time/#Parse) for more details on ``layout``. |
| <span class="opa-keep-it-together">``time.parse_rfc3339_ns(value, output)``</span> | 1 | ``output`` is ``number`` representing the time ``value`` in nanoseconds since epoch. |
| <span class="opa-keep-it-together">``time.parse_duration_ns(duration, output)``</span> | 1 | ``output`` is ``number`` representing the duration ``duration`` in nanoseconds. See the [Go `time` package documentation](https://golang.org/pkg/time/#ParseDuration) for more details on ``duration``. |
| <span class="opa-keep-it-together">``time.date(ns, [year, month, day])``</span> | 1 | outputs the ``year``, ``month`` (0-12), and ``day`` (0-31) as ``number``s representing the date from the nanoseconds since epoch (``ns``). |
| <span class="opa-keep-it-together">``time.clock(ns, [hour, minute, second])``</span> | 1 | outputs the ``hour``, ``minute`` (0-59), and ``second`` (0-59) as ``number``s representing the time of day for the nanoseconds since epoch (``ns``). |
| <span class="opa-keep-it-together">``time.weekday(ns, day)``</span> | 1 | outputs the ``day`` as ``string`` representing the day of the week for the nanoseconds since epoch (``ns``). |

> Multiple calls to the `time.now_ns` built-in function within a single policy
evaluation query will always return the same value.

### Cryptography

| Built-in | Inputs | Description |
| -------- | ------ | ----------- |
| <span class="opa-keep-it-together">``crypto.x509.parse_certificates(string, array[object])``</span> | 1 | ``output`` is an array of X.509 certificates represented as JSON objects. |

### Graphs

| Built-in | Inputs | Description |
| --- | --- | --- |
| <span class="opa-keep-it-together">``walk(x, [path, value])``</span> | 0 | ``walk`` is a relation that produces ``path`` and ``value`` pairs for documents under ``x``. ``path`` is ``array`` representing a pointer to ``value`` in ``x``.  Queries can use ``walk`` to traverse documents nested under ``x`` (recursively). |

### HTTP
| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``http.send(request, output)``</span> | 1 | ``http.send`` executes a HTTP request and returns the response.``request`` is an object containing keys ``method``, ``url`` and  optionally ``body`` and ``enable_redirect``. For example, ``http.send({"method": "get", "url": "http://www.openpolicyagent.org/"}, output)``. ``output`` is an object containing keys ``status``, ``status_code`` and ``body`` which represent the HTTP status, status code and response body respectively. Sample output, ``{"status": "200 OK", "status_code": 200, "body": null``}. By default, http redirects are not enabled. To enable, set ``enable_redirect`` to ``true``.|

### Net
| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``net.cidr_overlap(cidr, ip, output)``</span> | 2 | `output` is `true` if `ip` (e.g. `127.0.0.1`) overlaps with `cidr` (e.g. `127.0.0.1/24`) and false otherwise. Supports both IPv4 and IPv6 notations.|

### Rego
| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``rego.parse_module(filename, string, output)``</span> | 2 | ``rego.parse_module`` parses the input ``string`` as a Rego module and returns the AST as a JSON object ``output``. |

### Debugging
| Built-in | Inputs | Description |
| ------- |--------|-------------|
| <span class="opa-keep-it-together">``trace(string)``</span> | 1 | ``trace`` outputs the debug message ``string`` as a ``Note`` event in the query explanation. For example, ``trace("Hello There!")`` includes ``Note "Hello There!"`` in the query explanation. To print variables, use sprintf. For example, ``person = "Bob"; trace(sprintf("Hello There! %v", [person]))`` will emit ``Note "Hello There! Bob"``. |

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
true
with
```

## Grammar

Rego’s syntax is defined by the following grammar:

```
module          = package { import } policy
package         = "package" ref
import          = "import" package [ "as" var ]
policy          = { rule }
rule            = [ "default" ] rule-head { rule-body }
rule-head       = var [ "(" rule-args ")" ] [ "[" term "]" ] [ = term ]
rule-args       = term { "," term }
rule-body       = [ else [ = term ] ] "{" query "}"
query           = literal { ";" | [\r\n] literal }
literal         = ( expr | "not" expr ) { with-modifier }
with-modifier   = "with" term "as" term
instructions    = expr { ";" | [\r\n] expr }
expr            = term | expr-built-in | expr-infix
expr-built-in   = var [ "." var ] "(" [ term { , term } ] ")"
expr-infix      = [ term "=" ] term infix-operator term
term            = ref | var | scalar | array | object | set | array-compr
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
