---
title: Policy Reference
kind: documentation
weight: 4
toc: true
---

This document is the authoritative specification of the Rego policy language
(V1). All policies in OPA are written in Rego.

## Built-in Functions

The built-in functions for the language provide basic operations to manipulate
scalar values (e.g. numbers and strings), and aggregate functions that summarize
complex types.

### Comparison

| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``x == y``</span>   | ``x`` is equal to ``y`` |
| <span class="opa-keep-it-together">``x != y``</span>   | ``x`` is not equal to ``y`` |
| <span class="opa-keep-it-together">``x < y``</span>   | ``x`` is less than ``y`` |
| <span class="opa-keep-it-together">``x <= y``</span>   | ``x`` is less than or equal to ``y`` |
| <span class="opa-keep-it-together">``x > y``</span>   | ``x`` is greater than ``y`` |
| <span class="opa-keep-it-together">``x >= y``</span>   | ``x`` is greater than or equal to ``y`` |

### Numbers

| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``z := x + y``</span>   | ``z`` is the sum of ``x`` and ``y`` |
| <span class="opa-keep-it-together">``z := x - y``</span>  | ``z`` is the difference of ``x`` and ``y`` |
| <span class="opa-keep-it-together">``z := x * y``</span>   | ``z`` is the product of ``x`` and ``y`` |
| <span class="opa-keep-it-together">``z := x / y``</span>   | ``z`` is the quotient of ``x`` and ``y``  |
| <span class="opa-keep-it-together">``z := x % y``</span>   | ``z`` is the remainder from the division of ``x`` and ``y``  |
| <span class="opa-keep-it-together">``output := round(x)``</span>    | ``output`` is ``x`` rounded to the nearest integer |
| <span class="opa-keep-it-together">``output := abs(x)``</span>    | ``output`` is the absolute value of ``x`` |

### Aggregates

| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``output := count(collection_or_string)``</span> | ``output`` is the length of the object, array, set, or string provided as input |
| <span class="opa-keep-it-together">``output := sum(array_or_set)``</span> | ``output`` is the sum of the numbers in ``array_or_set`` |
| <span class="opa-keep-it-together">``output := product(array_or_set)``</span> | ``output`` is the product of the numbers in ``array_or_set`` |
| <span class="opa-keep-it-together">``output := max(array_or_set)``</span> | ``output`` is the maximum value in ``array_or_set`` |
| <span class="opa-keep-it-together">``output := min(array_or_set)``</span> | ``output`` is the minimum value in ``array_or_set`` |
| <span class="opa-keep-it-together">``output := sort(array_or_set)``</span> | ``output`` is the sorted ``array`` containing elements from ``array_or_set``. |
| <span class="opa-keep-it-together">``output := all(array_or_set)``</span> | ``output`` is ``true`` if all of the values in ``array_or_set`` are ``true``. A collection of length 0 returns ``true``.|
| <span class="opa-keep-it-together">``output := any(array_or_set)``</span> | ``output`` is ``true`` if any of the values in ``array_or_set`` is ``true``. A collection of length 0 returns ``false``.|

### Arrays
| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``output := array.concat(array, array)``</span> | ``output`` is the result of concatenating the two input arrays together. |
  <span class="opa-keep-it-together">``output := array.slice(array, startIndex, stopIndex)``</span> | ``output`` is the part of the ``array`` from ``startIndex`` to ``stopIndex`` including the first but excluding the last. If `startIndex >= stopIndex` then `output == []`. If both `startIndex` and `stopIndex` are less than zero, `output == []`. Otherwise, `startIndex` and `stopIndex` are clamped to 0 and `count(array)` respectively. |

### Sets

| Built-in | Description |
| -------- | ----------- |
| <span class="opa-keep-it-together">``s3 := s1 & s2``</span> | ``s3`` is the intersection of ``s1`` and ``s2``. |
| <span class="opa-keep-it-together"><code>s3 := s1 &#124; s2</code></span> | ``s3`` is the union of ``s1`` and ``s2``. |
| <span class="opa-keep-it-together">``s3 := s1 - s2``</span> | ``s3`` is the difference between ``s1`` and ``s2``, i.e., the elements in ``s1`` that are not in ``s2`` |
| <span class="opa-keep-it-together">``output := intersection(set[set])``</span> | ``output`` is the intersection of the sets in the input set  |
| <span class="opa-keep-it-together">``output := union(set[set])``</span> | ``output`` is the union of the sets in the input set  |

### Strings

| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``output := concat(delimiter, array_or_set)``</span> | ``output`` is the result of joining together the elements of ``array_or_set`` with the  string ``delimiter`` |
| <span class="opa-keep-it-together">``contains(string, search)``</span> | true if ``string`` contains ``search`` |
| <span class="opa-keep-it-together">``endswith(string, search)``</span> | true if ``string`` ends with ``search`` |
| <span class="opa-keep-it-together">``output := format_int(number, base)``</span> | ``output`` is string representation of ``number`` in the given ``base`` |
| <span class="opa-keep-it-together">``output := indexof(string, search)``</span> | ``output`` is the index inside ``string`` where ``search`` first occurs, or -1 if ``search`` does not exist |
| <span class="opa-keep-it-together">``output := lower(string)``</span> | ``output`` is ``string`` after converting to lower case |
| <span class="opa-keep-it-together">``output := replace(string, old, new)``</span> | ``output`` is a ``string`` representing ``string`` with all instances of ``old`` replaced by ``new`` |
| <span class="opa-keep-it-together">``output := strings.replace_n(patterns, string)``</span> | ``patterns`` is an object with old, new string key value pairs (e.g. ``{"old1": "new1", "old2": "new2", ...}``). ``output`` is a ``string`` with all old strings inside ``patterns`` replaced by the new strings |
| <span class="opa-keep-it-together">``output := split(string, delimiter)``</span> | ``output`` is ``array[string]`` representing elements of ``string`` separated by ``delimiter`` |
| <span class="opa-keep-it-together">``output := sprintf(string, values)``</span> | ``output`` is a ``string`` representing ``string`` formatted by the values in the ``array`` ``values``. |
| <span class="opa-keep-it-together">``startswith(string, search)``</span> | true if ``string`` begins with ``search`` |
| <span class="opa-keep-it-together">``output := substring(string, start, length)``</span> | ``output`` is the portion of ``string`` from index ``start`` and having a length of ``length``.  If ``length`` is less than zero, ``length`` is the remainder of the ``string``. If ``start`` is greater than the length of the string, ``output`` is empty. It is invalid to pass a negative offset to this function. |
| <span class="opa-keep-it-together">``output := trim(string, cutset)``</span> | ``output`` is a ``string`` representing ``string`` with all leading and trailing instances of the characters in ``cutset`` removed. |
| <span class="opa-keep-it-together">``output := trim_left(string, cutset)``</span> | ``output`` is a ``string`` representing ``string`` with all leading instances of the characters in ``cutset`` removed. |
| <span class="opa-keep-it-together">``output := trim_prefix(string, prefix)``</span> | ``output`` is a ``string`` representing ``string`` with leading instance of ``prefix`` removed. If ``string`` doesn't start with prefix, ``string`` is returned unchanged.|
| <span class="opa-keep-it-together">``output := trim_right(string, cutset)``</span> | ``output`` is a ``string`` representing ``string`` with all trailing instances of the characters in ``cutset`` removed. |
| <span class="opa-keep-it-together">``output := trim_suffix(string, suffix)``</span> | ``output`` is a ``string`` representing ``string`` with trailing instance of ``suffix`` removed. If ``string`` doesn't end with suffix, ``string`` is returned unchanged.|
| <span class="opa-keep-it-together">``output := trim_space(string)``</span> | ``output`` is a ``string`` representing ``string`` with all leading and trailing white space removed.|
| <span class="opa-keep-it-together">``output := upper(string)``</span> | ``output`` is ``string`` after converting to upper case |

### Regex
| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``re_match(pattern, value)``</span> | true if the ``value`` matches the regex ``pattern`` |
| <span class="opa-keep-it-together">``output := regex.split(pattern, string)``</span> | ``output`` is ``array[string]`` representing elements of ``string`` separated by ``pattern`` |
| <span class="opa-keep-it-together">``regex.globs_match(glob1, glob2)``</span> | true if the intersection of regex-style globs ``glob1`` and ``glob2`` matches a non-empty set of non-empty strings. The set of regex symbols is limited for this builtin: only ``.``, ``*``, ``+``, ``[``, ``-``, ``]`` and ``\`` are treated as special symbols. |
| <span class="opa-keep-it-normal">``output := regex.template_match(patter, string, delimiter_start, delimiter_end)``</span> | ``output`` is true if ``string`` matches ``pattern``. ``pattern`` is a string containing ``0..n`` regular expressions delimited by ``delimiter_start`` and ``delimiter_end``. Example ``regex.template_match("urn:foo:{.*}", "urn:foo:bar:baz", "{", "}", x)`` returns ``true`` for ``x``. |
| <span class="opa-keep-it-together">``output := regex.find_n(pattern, string, number)``</span> | ``output`` is an ``array[string]`` with the ``number`` of values matching the ``pattern``. A ``number`` of ``-1`` means all matches. |
| <span class="opa-keep-it-together">``output := regex.find_all_string_submatch_n(pattern, string, number)``</span> | ``output`` is an ``array[array[string]]`` with the outer `array` including a ``number`` of matches which match the ``pattern``. A ``number`` of ``-1`` means all matches. |

### Glob
| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``output := glob.match(pattern, delimiters, match)``</span> | ``output`` is true if ``match`` can be found in ``pattern`` which is separated by ``delimiters``. For valid patterns, check the table below. Argument ``delimiters`` is an array of single-characters (e.g. `[".", ":"]`). If ``delimiters`` is empty, it defaults to ``["."]``. |
| <span class="opa-keep-it-together">``output := glob.quote_meta(pattern)``</span> | ``output`` is the escaped string of ``pattern``. Calling ``glob.quote_meta("*.github.com", output)`` returns ``\\*.github.com`` as ``output``. |

The following table shows examples of how ``glob.match`` works:

| ``call`` | ``output`` | Description |
| -------- | ---------- | ----------- |
| ``output := glob.match("*.github.com", [], "api.github.com")`` | ``true`` | A glob with the default ``["."]`` delimiter. |
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
| ``output := glob.match(""{cat,bat,[fr]at}", [], "cat")`` | ``true`` | A glob with pattern-alternatives matchers. |
| ``output := glob.match(""{cat,bat,[fr]at}", [], "bat")`` | ``true`` | A glob with pattern-alternatives matchers. |
| ``output := glob.match(""{cat,bat,[fr]at}", [], "rat")`` | ``true`` | A glob with pattern-alternatives matchers. |
| ``output := glob.match(""{cat,bat,[fr]at}", [], "at")`` | ``false`` | A glob with pattern-alternatives matchers. |

### Conversions

| Built-in | Description |
| --- | --- |
| <span class="opa-keep-it-together">``output := to_number(x)``</span> | ``output`` is ``x`` converted to a number. `null` is converted to zero, `true` and `false` are converted to one and zero (respectively), `string` values are interpreted as base 10, and `numbers` are a no-op. Other types are not supported. |

### Units

| Built-in | Description |
| --- | --- |
| <span class="opa-keep-it-together">``output := units.parse_bytes(x)``</span> | ``output`` is ``x`` converted to a number with support for standard byte units (e.g., KB, KiB, etc.) KB, MB, GB, and TB are treated as decimal units and KiB, MiB, GiB, and TiB are treated as binary units. |

### Types

| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``output := is_number(x)``</span> | ``output`` is ``true`` if ``x`` is a number; otherwise undefined|
| <span class="opa-keep-it-together">``output := is_string(x)``</span> | ``output`` is ``true`` if ``x`` is a string; otherwise undefined |
| <span class="opa-keep-it-together">``output := is_boolean(x)``</span> | ``output`` is ``true`` if ``x`` is a boolean; otherwise undefined |
| <span class="opa-keep-it-together">``output := is_array(x)``</span> | ``output`` is ``true`` if ``x`` is an array; otherwise undefined |
| <span class="opa-keep-it-together">``output := is_set(x)``</span> | ``output`` is ``true`` if ``x`` is a set; otherwise undefined |
| <span class="opa-keep-it-together">``output := is_object(x)``</span> | ``output`` is ``true`` if ``x`` is an object; otherwise undefined |
| <span class="opa-keep-it-together">``output := is_null(x)``</span> | ``output`` is ``true`` if ``x`` is null; otherwise undefined |
| <span class="opa-keep-it-together">``output := type_name(x)``</span> | ``output`` is the type of ``x`` |

### Encoding

| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``output := base64.encode(x)``</span> | ``output`` is ``x`` serialized to a base64 encoded string |
| <span class="opa-keep-it-together">``output := base64.decode(string)``</span> | ``output`` is ``x`` deserialized from a base64 encoding string |
| <span class="opa-keep-it-together">``output := base64url.encode(x)``</span> | ``output`` is ``x`` serialized to a base64url encoded string |
| <span class="opa-keep-it-together">``output := base64url.decode(string)``</span> | ``output`` is ``string`` deserialized from a base64url encoding string |
| <span class="opa-keep-it-together">``output := urlquery.encode(string)``</span> | ``output`` is ``string`` serialized to a URL query parameter encoded string |
| <span class="opa-keep-it-together">``output := urlquery.encode_object(object)``</span> | ``output`` is ``object`` serialized to a URL query parameter encoded string |
| <span class="opa-keep-it-together">``output := urlquery.decode(string)``</span> | ``output`` is ``string`` deserialized from a URL query parameter encoded string |
| <span class="opa-keep-it-together">``output := json.marshal(x)``</span> | ``output`` is ``x`` serialized to a JSON string |
| <span class="opa-keep-it-together">``output := json.unmarshal(string)``</span> | ``output`` is ``string`` deserialized to a term from a JSON encoded string |
| <span class="opa-keep-it-together">``output := yaml.marshal(x)``</span> | ``output`` is ``x`` serialized to a YAML string |
| <span class="opa-keep-it-together">``output := yaml.unmarshal(string)``</span> | ``output`` is ``string`` deserialized to a term from YAML encoded string |

### Token Signing

OPA provides two builtins that implement JSON Web Signature [RFC7515](https://tools.ietf.org/html/rfc7515) functionality.

``io.jwt.encode_sign_raw()`` takes three JSON Objects (strings) as parameters and returns their JWS Compact Serialization. This builtin should be used by those that want maximum control over the signing and serialization procedure. It is important to remember that StringOrURI values are compared as case-sensitive strings with no transformations or canonicalizations applied. Therefore, line breaks and whitespaces are significant.

``io.jwt.encode_sign()`` takes three Rego Objects as parameters and returns their JWS Compact Serialization. This builtin should be used by those that want to use rego objects for signing during policy evaluation. Notice that since these are objects canonicalization *is applied* and all whitespace is removed.

The following algorithms are supported:

	ES256       "ES256" // ECDSA using P-256 and SHA-256
	ES384       "ES384" // ECDSA using P-384 and SHA-384
	ES512       "ES512" // ECDSA using P-521 and SHA-512
	HS256       "HS256" // HMAC using SHA-256
	HS384       "HS384" // HMAC using SHA-384
	HS512       "HS512" // HMAC using SHA-512
	NoSignature "none"
	PS256       "PS256" // RSASSA-PSS using SHA256 and MGF1-SHA256
	PS384       "PS384" // RSASSA-PSS using SHA384 and MGF1-SHA384
	PS512       "PS512" // RSASSA-PSS using SHA512 and MGF1-SHA512
	RS256       "RS256" // RSASSA-PKCS-v1.5 using SHA-256
	RS384       "RS384" // RSASSA-PKCS-v1.5 using SHA-384
	RS512       "RS512" // RSASSA-PKCS-v1.5 using SHA-512

<br>

| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``output := io.jwt.encode_sign_raw(headers, payload, key)``</span> | ``headers``, ``payload`` and  ``key`` as strings that represent the JWS Protected Header, JWS Payload and JSON Web Key ([RFC7517](https://tools.ietf.org/html/rfc7517)) respectively.|
``output := io.jwt.encode_sign(headers, payload, key)``</span> | ``headers``, ``payload`` and  ``key`` are JSON objects that represent the JWS Protected Header, JWS Payload and JSON Web Key ([RFC7517](https://tools.ietf.org/html/rfc7517)) respectively.|

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
io.jwt.encode_sign_raw(`{"typ":"JWT","alg":"HS256"}`, `{"iss":"joe","exp":1300819380,"http://example.com/is_root":true}`, `{"kty":"oct","k":"AyM1SysPpbyDfgZld3umj1qzKObwVMkoqQ-EstJQLr_T-1qS0gZH75aKtMN3Yj0iPS4hcgUuTwjAzZr1Z9CAow"}`)
```
```live:jwt/raw:output
```

### Token Verification

| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``output := io.jwt.verify_rs256(string, certificate)``</span> | ``output`` is ``true`` if the RS256 signature of the input token is valid. ``certificate`` is the PEM encoded certificate or the JWK key (set) used to verify the RS256 signature|
| <span class="opa-keep-it-together">``output := io.jwt.verify_ps256(string, certificate)``</span> | ``output`` is ``true`` if the PS256 signature of the input token is valid. ``certificate`` is the PEM encoded certificate or the JWK key (set) used to verify the PS256 signature|
| <span class="opa-keep-it-together">``output := io.jwt.verify_es256(string, certificate)``</span> | ``output`` is ``true`` if the ES256 signature of the input token is valid. ``certificate`` is the PEM encoded certificate or the JWK key (set) used to verify the ES256 signature|
| <span class="opa-keep-it-together">``output := io.jwt.verify_hs256(string, secret)``</span> | ``output`` is ``true`` if the Secret signature of the input token is valid. ``secret`` is a plain text secret used to verify the HS256 signature|
| <span class="opa-keep-it-together">``output := io.jwt.decode(string)``</span> | ``output`` is of the form ``[header, payload, sig]``.  ``header`` and ``payload`` are ``object``. ``sig`` is the hexadecimal representation of the signature on the token. |
| <span class="opa-keep-it-together">``output := io.jwt.decode_verify(string, constraints)``</span> | ``output`` is of the form ``[valid, header, payload]``.  If the input token verifies and meets the requirements of ``constraints`` then ``valid`` is ``true`` and ``header`` and ``payload`` are objects containing the JOSE header and the JWT claim set. Otherwise, ``valid`` is ``false`` and ``header`` and ``payload`` are ``{}``. |

The input `string` is a JSON Web Token encoded with JWS Compact Serialization. JWE and JWS JSON Serialization are not supported. If nested signing was used, the ``header``, ``payload`` and ``signature`` will represent the most deeply nested token.

For ``io.jwt.decode_verify``, ``constraints`` is an object with the following members:

| Name | Meaning | Required |
| ---- | ------- | -------- |
| ``cert`` | A PEM encoded certificate or a JWK key (set) containing an RSA or ECDSA public key. | See below |
| ``secret`` | The secret key for HS256, HS384 and HS512 verification. | See below |
| ``alg`` | The JWA algorithm name to use. If it is absent then any algorithm that is compatible with the key is accepted. | Optional |
| ``iss`` | The issuer string. If it is present the only tokens with this issuer are accepted. If it is absent then any issuer is accepted. | Optional |
|``time`` | The time in nanoseconds to verify the token at. If this is present then the ``exp`` and ``nbf`` claims are compared against this value. If it is absent then they are compared against the current time. | Optional |
|``aud`` | The audience that the verifier identifies with.  If this is present then the ``aud`` claim is checked against it. If it is absent then the ``aud`` claim must be absent too. | Optional |

Exactly one of ``cert`` and ``secret`` must be present. If there are any
unrecognized constraints then the token is considered invalid.


#### Token Verification Examples

The examples below use the following token:

```live:jwt/verify:module
es256_token = "eyJ0eXAiOiAiSldUIiwgImFsZyI6ICJFUzI1NiJ9.eyJuYmYiOiAxNDQ0NDc4NDAwLCAiaXNzIjogInh4eCJ9.lArczfN-pIL8oUU-7PU83u-zfXougXBZj6drFeKFsPEoVhy9WAyiZlRshYqjTSXdaw8yw2L-ovt4zTUZb2PWMg"
```

##### Using JWKS

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
io.jwt.verify_es256(es256_token, jwks)
[header, payload, _] := io.jwt.decode(es256_token)
payload.iss == "xxx"
```
```live:jwt/verify/jwks/two_step:output
```

```live:jwt/verify/jwks/one_step:query:merge_down
[valid, header, payload] := io.jwt.decode_verify(es256_token, {
    "cert": jwks,
    "iss": "xxx",
})
```
```live:jwt/verify/jwks/one_step:output
```

##### Using PEM encoded X.509 Certificate

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

```live:jwt/verify/cert/two_step:query:merge_down
io.jwt.verify_es256(es256_token, cert)
[header, payload, _] := io.jwt.decode(es256_token)
payload.iss == "xxx"
```
```live:jwt/verify/cert/two_step:output
```

```live:jwt/verify/cert/one_step:query:merge_down
[valid, header, payload] := io.jwt.decode_verify(es256_token, {
    "cert": cert,
    "iss": "xxx",
})
```
```live:jwt/verify/cert/one_step:output
```

### Time

| Built-in | Description |
| -------- | ----------- |
| <span class="opa-keep-it-together">``output := time.now_ns()``</span> | ``output`` is ``number`` representing the current time since epoch in nanoseconds. |
| <span class="opa-keep-it-together">``output := time.parse_ns(layout, value)``</span> | ``output`` is ``number`` representing the time ``value`` in nanoseconds since epoch. See the [Go `time` package documentation](https://golang.org/pkg/time/#Parse) for more details on ``layout``. |
| <span class="opa-keep-it-together">``output := time.parse_rfc3339_ns(value)``</span> | ``output`` is ``number`` representing the time ``value`` in nanoseconds since epoch. |
| <span class="opa-keep-it-together">``output := time.parse_duration_ns(duration)``</span> | ``output`` is ``number`` representing the duration ``duration`` in nanoseconds. See the [Go `time` package documentation](https://golang.org/pkg/time/#ParseDuration) for more details on ``duration``. |
| <span class="opa-keep-it-together">``output := time.date(ns)``<br/>``output := time.date([ns, tz])``</span> | ``output`` is of the form ``[year, month, day]``, which includes the ``year``, ``month`` (0-12), and ``day`` (0-31) as ``number``s representing the date from the nanoseconds since epoch (``ns``) in the timezone (``tz``), if supplied, or as UTC.|
| <span class="opa-keep-it-together">``output := time.clock(ns)``<br/>``output := time.clock([ns, tz])``</span> | ``output`` is of the form ``[hour, minute, second]``, which outputs the ``hour``, ``minute`` (0-59), and ``second`` (0-59) as ``number``s representing the time of day for the nanoseconds since epoch (``ns``) in the timezone (``tz``), if supplied, or as UTC. |
| <span class="opa-keep-it-together">``day := time.weekday(ns)``<br/>``day := time.weekday([ns, tz])``</span> | outputs the ``day`` as ``string`` representing the day of the week for the nanoseconds since epoch (``ns``) in the timezone (``tz``), if supplied, or as UTC. |

> Multiple calls to the `time.now_ns` built-in function within a single policy
evaluation query will always return the same value.

Timezones can be specified as

* an [IANA Time Zone](https://www.iana.org/time-zones) string e.g. "America/New_York"
* "UTC" or "", which are equivalent to not passing a timezone (i.e. will return as UTC)
* "Local", which will use the local timezone.

Note that the opa executable will need access to the timezone files in the environment it is running in (see the [Go time.LoadLocation()](https://golang.org/pkg/time/#LoadLocation) documentation for more information).

### Cryptography

| Built-in | Description |
| -------- | ----------- |
| <span class="opa-keep-it-together">``output := crypto.x509.parse_certificates(string)``</span> | ``output`` is an array of X.509 certificates represented as JSON objects. |

### Graphs

| Built-in | Description |
| --- | --- |
| <span class="opa-keep-it-together">``walk(x, [path, value])``</span> | ``walk`` is a relation that produces ``path`` and ``value`` pairs for documents under ``x``. ``path`` is ``array`` representing a pointer to ``value`` in ``x``.  Queries can use ``walk`` to traverse documents nested under ``x`` (recursively). |

### HTTP

| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``http.send(request, output)``</span> | ``http.send`` executes a HTTP request and returns the response.``request`` is an object containing keys ``method``, ``url`` and  optionally ``body``, ``enable_redirect``, ``force_json_decode``, ``headers``, ``tls_use_system_certs``, ``tls_ca_cert_file``, ``tls_ca_cert_env_variable``, ``tls_client_cert_env_variable``, ``tls_client_key_env_variable`` or ``tls_client_cert_file``, ``tls_client_key_file`` . For example, ``http.send({"method": "get", "url": "http://www.openpolicyagent.org/", "headers": {"X-Foo":"bar", "X-Opa": "rules"}}, output)``. ``output`` is an object containing keys ``status``, ``status_code``, ``body`` and ``raw_body`` which represent the HTTP status, status code, JSON value from the response body and response body as string respectively. Sample output, ``{"status": "200 OK", "status_code": 200, "body": {"hello": "world"}, "raw_body": "{\"hello\": \"world\"}"``}. By default, HTTP redirects are not enabled. To enable, set ``enable_redirect`` to ``true``. Also ``force_json_decode`` is set to ``false`` by default. This means if the HTTP server response does not specify the ``Content-type`` as ``application/json``, the response body will not be JSON decoded ie. output's ``body`` field will be ``null``. To change this behaviour, set ``force_json_decode`` to ``true``.|

#### HTTPs Usage

The following table explains the HTTPs objects

| Object |  Definition | Value|
| -------- |-----------|------|
| tls_use_system_certs | Use system certificate pool | true or false
| tls_ca_cert_file | Path to file containing a root certificate in PEM encoded format | double-quoted string
| tls_ca_cert_env_variable | Environment variable containing a root certificate in PEM encoded format | double-quoted string
| tls_client_cert_env_variable | Environment variable containing a client certificate in PEM encoded format | double-quoted string
| tls_client_key_env_variable | Environment variable containing a client key in PEM encoded format | double-quoted string
| tls_client_cert_file | Path to file containing a client certificate in PEM encoded format | double-quoted string
| tls_client_key_file | Path to file containing a key  in PEM encoded format | double-quoted string

In order to trigger the use of HTTPs the user must provide one of the following combinations:

 * ``tls_client_cert_file``, ``tls_client_key_file``
 * ``tls_client_cert_env_variable``, ``tls_client_key_env_variable``

 The user must also provide a trusted root CA through tls_ca_cert_file or tls_ca_cert_env_variable. Alternatively the user could set tls_use_system_certs to ``true`` and the system certificate pool will be used.

#### HTTPs Examples

| Examples |  Comments |
| -------- |-----------|
| Files containing TLS material | ``http.send({"method": "get", "url": "https://127.0.0.1:65331", "tls_ca_cert_file": "testdata/ca.pem", "tls_client_cert_file": "testdata/client-cert.pem", "tls_client_key_file": "testdata/client-key.pem"}, output)``.
|Environment variables containing TLS material | ``http.send({"method": "get", "url": "https://127.0.0.1:65360", "tls_ca_cert_env_variable": "CLIENT_CA_ENV", "tls_client_cert_env_variable": "CLIENT_CERT_ENV", "tls_client_key_env_variable": "CLIENT_KEY_ENV"}, output)``.|
| Accessing Google using System Cert Pool | ``http.send({"method": "get", "url": "https://www.google.com", "tls_use_system_certs": true, "tls_client_cert_file": "testdata/client-cert.pem", "tls_client_key_file": "testdata/client-key.pem"}, output)``

### Net
| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``net.cidr_contains(cidr, cidr_or_ip)``</span> | `output` is `true` if `cidr_or_ip` (e.g. `127.0.0.64/26` or `127.0.0.1`) is contained within `cidr` (e.g. `127.0.0.1/24`) and false otherwise. Supports both IPv4 and IPv6 notations.|
| <span class="opa-keep-it-together">``net.cidr_intersects(cidr1, cidr2)``</span> | `output` is `true` if `cidr1` (e.g. `192.168.0.0/16`) overlaps with `cidr2` (e.g. `192.168.1.0/24`) and false otherwise. Supports both IPv4 and IPv6 notations.|

### Rego
| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``output := rego.parse_module(filename, string)``</span> | ``rego.parse_module`` parses the input ``string`` as a Rego module and returns the AST as a JSON object ``output``. |

### OPA
| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``output := opa.runtime()``</span> | ``opa.runtime`` returns a JSON object ``output`` that describes the runtime environment where OPA is deployed. **Caution**: Policies that depend on the output of ``opa.runtime`` may return different answers depending on how OPA was started. If possible, prefer using an explicit `input` or `data` value instead of `opa.runtime`. The ``output`` of ``opa.runtime`` will include a ``"config"`` key if OPA was started with a configuration file. The ``output`` of ``opa.runtime`` will include a ``"env"`` key containing the environment variables that the OPA process was started with. The ``output`` of ``opa.runtime`` will include ``"version"`` and ``"commit"`` keys containing the semantic version and build commit of OPA. |

### Debugging
| Built-in | Description |
| ------- |-------------|
| <span class="opa-keep-it-together">``trace(string)``</span> | ``trace`` outputs the debug message ``string`` as a ``Note`` event in the query explanation. For example, ``trace("Hello There!")`` includes ``Note "Hello There!"`` in the query explanation. To print variables, use sprintf. For example, ``person := "Bob"; trace(sprintf("Hello There! %v", [person]))`` will emit ``Note "Hello There! Bob"``. |

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
rule-head       = var [ "(" rule-args ")" ] [ "[" term "]" ] [ ( ":=" / "=" ) term ]
rule-args       = term { "," term }
rule-body       = [ else [ = term ] ] "{" query "}"
query           = literal { ";" | [\r\n] literal }
literal         = ( some-decl | expr | "not" expr ) { with-modifier }
with-modifier   = "with" term "as" term
some-decl       = "some" var { "," var }
expr            = term | expr-built-in | expr-infix
expr-built-in   = var [ "." var ] "(" [ term { , term } ] ")"
expr-infix      = [ term "=" ] term infix-operator term
term            = ref | var | scalar | array | object | set | array-compr | object-compr | set-compr
array-compr     = "[" term "|" rule-body "]"
set-compr       = "{" term "|" rule-body "}"
object-compr    = "{" object-item "|" rule-body "}"
infix-operator  = bool-operator | arith-operator | bin-operator
bool-operator   = "==" | "!=" | "<" | ">" | ">=" | "<="
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
```
