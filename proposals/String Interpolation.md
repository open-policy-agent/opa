# **Design Document: String Interpolation in Rego**

# 1. Introduction

This document outlines the design for introducing string interpolation to the Rego programming language used in the Open Policy Agent (OPA). String interpolation will provide a more concise and readable way to construct strings that include dynamic values.

Currently, Rego supports two types of strings:

* string (`“<text>”`): for regular strings (single-line, escaped special characters)  
* raw-string (`` `<text>` ``): for raw strings (multi-line, no escaping)

Here, we introduce two new types of interpolated strings, that can contain expressions calculated at eval-time:

* template-string (`$”<text>”`): (single-line, escaped special characters)  
* raw-template-string (`` $`<text>` ``): (multi-line, no escaping)

By introducing new string types that use a new syntax, backwards-compatibility with already existing policies is guaranteed.

# 2. Goals

* Enhance readability and maintainability of Rego policies.  
* Simplify the creation of strings with embedded variables.  
* Undefined values don’t abort expression evaluation  
* Minimize changes to existing Rego syntax and semantics.  
* Ensure backward compatibility with existing policies.

# 3. Glossary

* **Template string**: a Rego Value type representing a string containing value-generating expressions (template expressions) calculated at eval-time; for supporting string interpolation.  
* **Template expression**: A value-generating Rego expression inside a template string.

# 4. Proposed Syntax

## 4.1. Terminal Symbols

The current rego syntax already supports two different string terminal symbols:

* Double quote (`“`): for regular strings (single-line, escaped special characters)  
* Backtick (`` ` ``): for raw strings (multi-line, no escaping)

To be non-breaking with the current Rego syntax and not change the semantics of existing policies, interpolated strings must be declared in a separate syntax than regular strings (`“<text>”`) and raw-strings (`` `<text>` ``). 

Here, we introduce two new, template string types, orthogonal to the existing types:

* template-string (`$”<text>”`): (single-line, escaped special characters)  
* raw-template-string (`` $`<text>` ``): (multi-line, no escaping)

Where we let a dollar-sign (`$`) prefix to the leading string terminal denote a template string.

```
package example

# input: {"user": Alice}, output: "Hello, Alice!"
p := $"Hello, {input.user}!" 
```

Existing string syntax (`“”`) is unaffected:

```
package example

# input: {"user": Alice}, output: "Hello, {input.user}!"
p := "Hello, {input.user}!" 
```

Multi-line template strings is supported by using backticks (``` $`` ```):

```
package example

# input: {"user": Alice}, output: 
# "<user>
#   <name>Alice</name>
# </user>"
p := $`<user>
  <name>{input.user}</name>
</user>`
```

## 4.2. Template Expression

The template expression resides inside a template string, is enclosed by opening and closing curly-braces (`{}`), and the contained expression is dynamically calculated at eval-time.

### 4.2.1. Escaping

Placeholder expressions can be escaped by prefixing the opening curly-brace (`{)` with a backslash (`\`): `\{`.

```
# output: "Hello, {n1} and Bob!"
p := $"Hello, \{n1} and {n2}!" if {
	n1 := "Alice"
	n2 := "Bob"
}
```

Closing curly-braces that don’t have a matching opening brace don’t require escaping; however, superfluous escaping isn’t rejected:

```
# output: "Hello, {n1} and Bob!"
p := $"Hello, \{n1\} and {n2}!" if {
	n1 := "Alice"
	n2 := "Bob"
}
```

Escaping an expression-closing curly-brace will however cause a parse-time error:

```
# output: rego_parse_error
p := $"Hello, {n1\}!" if {
	n1 := "Alice"
}
```

String terminals and other operators inside of a template expression are **not** escaped:

```
# output: rego_parse_error
p := $"Hello, {\"Alice\"}!"
```

### 4.2.2 Primitives

Booleans:

```
# output: "False is not true!"
p := $"False is not {true}!"
```

Numbers:

```
# output: "Shoe size is 42!"
p := $"Shoe size is {42}!"
```

Strings:

```
# output: "Hello Alice!"
p := $"Hello, {"Alice"}!"
```

Arrays:

```
# output: "Hello [\"Alice\"]!"
p := $"Hello, {["Alice", "Bob"]}!"
```

Sets:

```
# output: "Hello {\"Alice\"}!"
p := $"Hello, {{"Alice"}}!"
```

Objects:

```
# output: "Hello {\"name\": \"Alice\"}!"
p := $"Hello, {{"name": "Alice"}}!"
```

### 4.2.3. Variables

```
# output: "Hello Alice!"
p := msg if {
	name := Alice
	msg := $"Hello, {name}!"
}
```

```
# output: "Hello Alice!"
p := $"Hello, {name}!" if {
	name := "Alice"
}
```

### 4.2.4. References

```
# input: {"name": "Alice"}
# output: "Hello Alice!"
p := $"Hello, {input.name}!"
```

```
users := [
{"name": "Alice"},
{"name": "Bob"},
]

# output: "Hello Bob!"
p := $"Hello, {users[1].name}!"
```

### 4.2.5. Comprehensions

Comprehensions generate the same output as their static value counterparts.

```
# output: "Numbers [0, 2, 4] are even"
p := $"Numbers {[x | some x in numbers.range(0, 5); x % 2 == 0]} are even"

```

```
# output: Hello, Alice!
p := $"Hello, {[x | x := "Alice"][0]}!"
```

### 4.2.6. Enumeration

Template expressions must generate a single value. Enumeration is not supported.

```
# output: rego_compile_error
p := $"Hello, {["Alice", "Bob"][_]}!"
```

```
# output: rego_compile_error
p contains $"Hello, {["Alice", "Bob"][_]}!"
```

Possible to support in the future.

### 4.2.7. Multi-line expressions

Template expressions can be multi-line regardless if they reside inside a template-string or a raw-timeplate-string (multi-line).

```
# output: Hello, [\"Alice\", \"Bob\"]!
p := $"Hello, {[x | 
some x in ["Alice", "Bob"]
]}!"
```

```
# output: Hello, [\"Alice\", \"Bob\"]!
p := $`Hello, {[x | 
some x in ["Alice", "Bob"]
]}!`
```

### 4.2.8. Nested template strings

Nonsensical in most use-cases:

```
# input: {"name": "Alice"}
# output: "Hello Alice!"
p := $"Hello {$"{input.name}!"}"
```

But can be useful inside comprehensions, where more complex expressions can be composed:

```
users := [
  {"name": "Alice", "surname": "Alisson"},
  {"name": "Bob", "surname": "Bobsson"},
]

# output: "Hello [\"Alice Allisson\", \"Bob Bobsson\"]!"
p := $`Hello {[name | 
  some user in users
  name := $"{user.name} {user.surname}"
]}!`
```

### 4.2.9. Undefined 

Template expressions that evaluate to **undefined** will not stop evaluation of its closure. The template expression will instead output the default string: `“<undefined>”`.

```
# input: {}
# output: “Hello, <undefined>. How are you?”
p := $”Hello, {input.user}. How are you?”
```

## 4.3. Multi-line

Supported through the raw-template-string type.

```
# input: {"name": "Alice", "surname": "Alisson"}
# output:
# <user>
#   <name>Alice</name>
#   <surname>Alisson</surname>
#</user>
p :=$`<user>
   <name>{input.name}</name>
   <surname>{input.surname}</surname>
</user>`
```

## 4.4. Grammar

* WiP, pending decisions in other sections

String interpolation adds the following rules to the [standard Rego grammar](https://www.openpolicyagent.org/docs/latest/policy-reference/#grammar):

```
tmpl-str-open = '$"' 
tmpl-str-close = '"'
raw-tmpl-str-open = '$`' 
raw-tmpl-str-close = '`'

# Rego 'term' rule excluding 'membership'
tmpl-str-expr = ref | var | scalar | array | object | set | array-compr | object-compr | set-compr | template-string
tmpl-str-placeholder = "{", tmpl-str-expr, "}"
tmpl-str-char-forbidden = tmpl-str-open | raw-tmpl-str-open | tmpl-str-close | raw-tmpl-str-close | tmpl-str-placeholder
tmpl-str-char = CHAR - dyn-str-char-forbidden
tmpl-str-char-seq = {dyn-str-char}
tmpl-str-content = tmpl-str-char-seq | [tmpl-str-char-seq], tmpl-str-placeholder, [dyn-str-char-seq]
template-string = (tmpl-str-open, {tmpl-str-content}, tmpl-str-close) / (raw-tmpl-str-open, {tmpl-str-content}, raw-tmpl-str-close)
```

### 4.4.1 Example (terminator escaping not required inside placeholder expr)

```
# output: "Hello, Alice!"
p := "Hello, %{"Alice"}!"
```

### 4.4.2 Example (placeholder escaping not required inside placeholder expr)

* Support nested template strings?

```
# output: "Hello, <Alice>!"
p := "Hello, %{"<%{"Alice"}>"}!"
```

## 4.5 Review of Other Languages

### 4.5.1 Java

* [https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/lang/StringTemplate.html](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/lang/StringTemplate.html)  
* The StringTemplate implementation was in preview in Java 21 and 22, removed in Java 23\.  
* Terminals: quotation marks (`“”`) with `STR.` prefix (static package ref)  
* Placeholders: `\{}`

```java
int x = 10;
int y = 20;
String s = STR."\{x} + \{y} = \{x + y}";
```

### 4.5.2 JavaScript

* [https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Template\_literals](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Template_literals)  
* Template literals  
* Terminals: backticks (``` `` ```)  
* Placeholders: `${}`  
* Multiline

```javascript
const x = 10;
const y = 20;
const s = `${x} + ${y} = ${x + y}`;
```

* Tagged templates ([https://wesbos.com/tagged-template-literals](https://wesbos.com/tagged-template-literals))  
  * Custom template functions

```javascript
function highlight() {
  return 'cool';
}


const sentence = highlight`My dog's name is ${name} and he is ${age} years old`;
console.log(sentence); // 'cool'
```

### 4.5.3 C\#

* [https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/tokens/interpolated](https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/tokens/interpolated)  
* Terminals: Quotation marks (`“`) with dollar (`$`) prefix  
* Placeholders: Curly braces (`{}`)  
* Placeholder can contain formatting directives: `{<interpolationExpression>[,<width>][:<formatString>]}`

```c#
var x = 10;
var y = 20;
var s = $"{x} + {y} = {x + y}";
```

### 4.5.4 Bash

* Terminals: Quotation marks (`“`)  
* Placeholders:  
  * Dollar sign for simple variable dereference ($)  
  * Curly braces with dollar sign prefix for complex expressions (`${}`)

```shell
echo "$x + $y = ${x + y}"
```

### 4.5.5 Dart

* Terminals: Single quotation marks (`‘`)  
* Placeholders:  
  * Dollar sign for simple variable dereference ($)  
  * Curly braces with dollar sign prefix for complex expressions (`${}`)

```shell
echo '$x + $y = ${x + y}'
```

### 4.5.6 Python

* Formatted String Literals (f-strings)  
* Terminals: Single quotation marks with f/F prefix (`f‘`)  
* Placeholders: Curly braces (`{}`)

```py
print(f'{x} + {y} = {x + y}')
```

# 5. Semantics

* Strings enclosed in backticks will be treated as interpolated values.  
  * `$“Hello, {“Alice”}. How are you?”` \-\> `“Hello, Alice. How are you?”`  
* `{}` will act as a placeholder for expressions.  
* Expressions inside `{}` will be evaluated and their string representation will be inserted into the string.  
* If the expression inside `{}` evaluates to a non-string value, it will be automatically converted to a string.  
* Expressions inside `{}` that evaluates to **undefined** will not stop evaluation of its closure.   
  * The template expression will instead output some default string.  
    * `$”Hello, {input.user}. How are you?”` \-\> `“Hello, <TBD>. How are you?”`  
      * `Hello, {input.user}. How are you?”`  
        * Keep template expressions. Dangerous; exposes code to the client.  
      * `Hello, {undefined}. How are you?”`  
        * Replace with `{undefined}`. Gives clear indication of something missing  
      * `Hello, . How are you?”`  
        * Omit output. Might make it hard to see that something is missing.  
* Regular strings enclosed in double quotes will remain unchanged.

# 6. Implementation Details

The `print()` built-in is prior art.

## 6.1. Parser

A new `ast.Value` type is created: `ast.TemplateString`.

E.g.

```go
type TemplateString []ast.Value
```

Where `ast.TemplateString` is a slice of `ast.Value` elements. Parsing splits the template string by containing placeholder expressions, where each expression and intermittent string becomes an entry in the slice.

`$”Hello, {input.name}!”` \-\>   
`ast.TemplateString{“Hello, ”, ast.Ref{ast.VarTerm("input"), ast.StringTerm{“name”}}, “!”}`

## 6.2. Compiler

Undefined values are handled by wrapping each template expression of the template string in individual set comprehensions (similar to how the `print()` built-in is handled).

Before compiler:

```
package example

p := "%{input.name} is %{input.age} years old"
```

After compiler:

```
package example

p = __local4__ if {
	__local2__ = {__local0__ | __local0__ = input.name}
	__local3__ = {__local1__ | __local1__ = input.age}
	internal.template_sprint([__local2__, " is ", __local3__, " years old"], __local4__)
}
```

## 6.3. Topdown

* Add `internal.template_sprint()` built-in.  
  * Concatenate elements in array arg  
    * String elements added as-is  
    * Set elements  
      * If non-empty: concatenate first entry  
      * If empty: concatenate `“<undefined>”` string.  
      * Any other value type: error

## 6.4. Planner

* Plan evaluators might need to implement `internal.template_sprint()`

## 6.5. Capabilities

* Capability: string\_interpolation  
  * Required in parser to parse template strings  
  * Required in compiler for 

## 6.6. Error Handling

Template expressions are treated as any other Rego expression, and are subject to all existing error conditions; e.g. type checking, undeclared vars, invalid assignments, etc.

# 7\. Thoughts

*  C-style format sub-specifiers  
  * [https://cplusplus.com/reference/cstdio/printf/](https://cplusplus.com/reference/cstdio/printf/)  
  * justification/padding/width  
  * Could be accomplished through complex placeholder expressions (e.g. pipe through sprintf)   
  * C\# supports this: `{<interpolationExpression>[,<width>][:<formatString>]}`  
* Step-by-step implementation possible  
  * A first release could implement simpler expressions, such as supporting only scalar values, variables and refs. Support for complex expressions, such as comprehensions and multi-line strings, could be added in subsequent releases.  
* As an alternative to single-entry set-comprehensions for individual arguments when calling `internal.template_sprint()`, an `ast.Optional` AST type could be introduced.  
  * Never undefined, like comprehensions  
  * Only a single entry is allowed; no ambiguity where set could contain multiple entries  
  * Treat as set in all situations where distinction between `ast.Optional` and `ast.Set` is irrelevant

