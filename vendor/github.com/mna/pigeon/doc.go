/*
Command pigeon generates parsers in Go from a PEG grammar.

From Wikipedia [0]:

	A parsing expression grammar is a type of analytic formal grammar, i.e.
	it describes a formal language in terms of a set of rules for recognizing
	strings in the language.

Its features and syntax are inspired by the PEG.js project [1], while
the implementation is loosely based on [2]. Formal presentation of the
PEG theory by Bryan Ford is also an important reference [3]. An introductory
blog post can be found at [4].

	[0]: http://en.wikipedia.org/wiki/Parsing_expression_grammar
	[1]: http://pegjs.org/
	[2]: http://www.codeproject.com/Articles/29713/Parsing-Expression-Grammar-Support-for-C-Part
	[3]: http://pdos.csail.mit.edu/~baford/packrat/popl04/peg-popl04.pdf
	[4]: http://0value.com/A-PEG-parser-generator-for-Go

Command-line usage

The pigeon tool must be called with PEG input as defined
by the accepted PEG syntax below. The grammar may be provided by a
file or read from stdin. The generated parser is written to stdout
by default.

	pigeon [options] [GRAMMAR_FILE]

The following options can be specified:

	-cache : cache parser results to avoid exponential parsing time in
	pathological cases. Can make the parsing slower for typical
	cases and uses more memory (default: false).

	-debug : boolean, print debugging info to stdout (default: false).

	-no-recover : boolean, if set, do not recover from a panic. Useful
	to access the panic stack when debugging, otherwise the panic
	is converted to an error (default: false).

	-o=FILE : string, output file where the generated parser will be
	written (default: stdout).

	-x : boolean, if set, do not build the parser, just parse the input grammar
	(default: false).

	-receiver-name=NAME : string, name of the receiver variable for the generated
	code blocks. Non-initializer code blocks in the grammar end up as methods on the
	*current type, and this option sets the name of the receiver (default: c).

If the code blocks in the grammar (see below, section "Code block") are golint-
and go vet-compliant, then the resulting generated code will also be golint-
and go vet-compliant.

The generated code doesn't use any third-party dependency unless code blocks
in the grammar require such a dependency.

PEG syntax

The accepted syntax for the grammar is formally defined in the
grammar/pigeon.peg file, using the PEG syntax. What follows is an informal
description of this syntax.

Identifiers, whitespace, comments and literals follow the same
notation as the Go language, as defined in the language specification
(http://golang.org/ref/spec#Source_code_representation):

	// single line comment*/
//	/* multi-line comment */
/*	'x' (single quotes for single char literal)
	"double quotes for string literal"
	`backtick quotes for raw string literal`
	RuleName (a valid identifier)

The grammar must be Unicode text encoded in UTF-8. New lines are identified
by the \n character (U+000A). Space (U+0020), horizontal tabs (U+0009) and
carriage returns (U+000D) are considered whitespace and are ignored except
to separate tokens.

Rules

A PEG grammar consists of a set of rules. A rule is an identifier followed
by a rule definition operator and an expression. An optional display name -
a string literal used in error messages instead of the rule identifier - can
be specified after the rule identifier. E.g.:
	RuleA "friendly name" = 'a'+ // RuleA is one or more lowercase 'a's

The rule definition operator can be any one of those:
	=, <-, ← (U+2190), ⟵ (U+27F5)

Expressions

A rule is defined by an expression. The following sections describe the
various expression types. Expressions can be grouped by using parentheses,
and a rule can be referenced by its identifier in place of an expression.

Choice expression

The choice expression is a list of expressions that will be tested in the
order they are defined. The first one that matches will be used. Expressions
are separated by the forward slash character "/". E.g.:
	ChoiceExpr = A / B / C // A, B and C should be rules declared in the grammar

Because the first match is used, it is important to think about the order
of expressions. For example, in this rule, "<=" would never be used because
the "<" expression comes first:
	BadChoiceExpr = "<" / "<="

Sequence expression

The sequence expression is a list of expressions that must all match in
that same order for the sequence expression to be considered a match.
Expressions are separated by whitespace. E.g.:
	SeqExpr = "A" "b" "c" // matches "Abc", but not "Acb"

Labeled expression

A labeled expression consists of an identifier followed by a colon ":"
and an expression. A labeled expression introduces a variable named with
the label that can be referenced in the code blocks in the same scope.
The variable will have the value of the expression that follows the colon.
E.g.:
	LabeledExpr = value:[a-z]+ {
		fmt.Println(value)
		return value, nil
	}

The variable is typed as an empty interface, and the underlying type depends
on the following:

For terminals (character and string literals, character classes and
the any matcher), the value is []byte. E.g.:
    Rule = label:'a' { // label is []byte }

For predicates (& and !), the value is always nil. E.g.:
	Rule = label:&'a' { // label is nil }

For a sequence, the value is a slice of empty interfaces, one for each
expression value in the sequence. The underlying types of each value
in the slice follow the same rules described here, recursively. E.g.:
	Rule = label:('a' 'b') { // label is []interface{} }

For a repetition (+ and *), the value is a slice of empty interfaces, one for
each repetition. The underlying types of each value in the slice follow
the same rules described here, recursively. E.g.:
	Rule = label:[a-z]+ { // label is []interface{} }

For a choice expression, the value is that of the matching choice. E.g.:
	Rule = label:('a' / 'b') { // label is []byte }

For the optional expression (?), the value is nil or the value of the
expression. E.g.:
	Rule = label:'a'? { // label is nil or []byte }

Of course, the type of the value can be anything once an action code block
is used. E.g.:
	RuleA = label:'3' {
		return 3, nil
	}
	RuleB = label:RuleA { // label is int }

And and not expressions

An expression prefixed with the ampersand "&" is the "and" predicate
expression: it is considered a match if the following expression is a match,
but it does not consume any input.

An expression prefixed with the exclamation point "!" is the "not" predicate
expression: it is considered a match if the following expression is not
a match, but it does not consume any input. E.g.:
	AndExpr = "A" &"B" // matches "A" if followed by a "B" (does not consume "B")
	NotExpr = "A" !"B" // matches "A" if not followed by a "B" (does not consume "B")

The expression following the & and ! operators can be a code block. In that
case, the code block must return a bool and an error. The operator's semantic
is the same, & is a match if the code block returns true, ! is a match if the
code block returns false. The code block has access to any labeled value
defined in its scope. E.g.:
	CodeAndExpr = value:[a-z] &{
		// can access the value local variable...
		return true, nil
	}

Repeating expressions

An expression followed by "*", "?" or "+" is a match if the expression
occurs zero or more times ("*"), zero or one time "?" or one or more times
("+") respectively. The match is greedy, it will match as many times as
possible. E.g.
	ZeroOrMoreAs = "A"*

Literal matcher

A literal matcher tries to match the input against a single character or a
string literal. The literal may be a single-quoted single character, a
double-quoted string or a backtick-quoted raw string. The same rules as in Go
apply regarding the allowed characters and escapes.

The literal may be followed by a lowercase "i" (outside the ending quote)
to indicate that the match is case-insensitive. E.g.:
	LiteralMatch = "Awesome\n"i // matches "awesome" followed by a newline

Character class matcher

A character class matcher tries to match the input against a class of characters
inside square brackets "[...]". Inside the brackets, characters represent
themselves and the same escapes as in string literals are available, except
that the single- and double-quote escape is not valid, instead the closing
square bracket "]" must be escaped to be used.

Character ranges can be specified using the "[a-z]" notation. Unicode
classes can be specified using the "[\pL]" notation, where L is a
single-letter Unicode class of characters, or using the "[\p{Class}]"
notation where Class is a valid Unicode class (e.g. "Latin").

As for string literals, a lowercase "i" may follow the matcher (outside
the ending square bracket) to indicate that the match is case-insensitive.
A "^" as first character inside the square brackets indicates that the match
is inverted (it is a match if the input does not match the character class
matcher). E.g.:
	NotAZ = [^a-z]i

Any matcher

The any matcher is represented by the dot ".". It matches any character
except the end of file, thus the "!." expression is used to indicate "match
the end of file". E.g.:
	AnyChar = . // match a single character
	EOF = !.

Code block

Code blocks can be added to generate custom Go code. There are three kinds
of code blocks: the initializer, the action and the predicate. All code blocks
appear inside curly braces "{...}".

The initializer must appear first in the grammar, before any rule. It is
copied as-is (minus the wrapping curly braces) at the top of the generated
parser. It may contain function declarations, types, variables, etc. just
like any Go file. Every symbol declared here will be available to all other
code blocks.  Although the initializer is optional in a valid grammar, it is
usually required to generate a valid Go source code file (for the package
clause). E.g.:
	{
		package main

		func someHelper() {
			// ...
		}
	}

Action code blocks are code blocks declared after an expression in a rule.
Those code blocks are turned into a method on the "*current" type in the
generated source code. The method receives any labeled expression's value
as argument (as interface{}) and must return two values, the first being
the value of the expression (an interface{}), and the second an error.
If a non-nil error is returned, it is added to the list of errors that the
parser will return. E.g.:
	RuleA = "A"+ {
		// return the matched string, "c" is the default name for
		// the *current receiver variable.
		return string(c.text), nil
	}

Predicate code blocks are code blocks declared immediately after the and "&"
or the not "!" operators. Like action code blocks, predicate code blocks
are turned into a method on the "*current" type in the generated source code.
The method receives any labeled expression's value as argument (as interface{})
and must return two values, the first being a bool and the second an error.
If a non-nil error is returned, it is added to the list of errors that the
parser will return. E.g.:
	RuleAB = [ab]i+ &{
		return true, nil
	}

The current type is a struct that provides three useful fields that can be
accessed in action and predicate code blocks: "pos", "text" and "globalStore".

The "pos" field indicates the current position of the parser in the source
input. It is itself a struct with three fields: "line", "col" and "offset".
Line is a 1-based line number, col is a 1-based column number that counts
runes from the start of the line, and offset is a 0-based byte offset.

The "text" field is the slice of bytes of the current match. It is empty
in a predicate code block.

The "globalStore" field is a global store of type "map[string]interface{}",
which allows to store arbitrary values, which are available in action and
predicate code blocks for read as well as write access.
It is important to notice, that the global store is completely independent from
the backtrack mechanism of PEG and is therefore not set back to its old state
during backtrack.
The initialization of the global store may be achieved by using the GlobalStore
function (http://godoc.org/github.com/mna/pigeon/test/predicates#GlobalStore).
Be aware, that all keys starting with "_pigeon" are reserved for internal use
of pigeon and should not be used nor modified. Those keys are treated as
internal implementation details and therefore there are no guarantees given in
regards of API stability.

Using the generated parser

The parser generated by pigeon exports a few symbols so that it can be used
as a package with public functions to parse input text. The exported API is:
	- Parse(string, []byte, ...Option) (interface{}, error)
	- ParseFile(string, ...Option) (interface{}, error)
	- ParseReader(string, io.Reader, ...Option) (interface{}, error)
	- Debug(bool) Option
	- GlobalStore(string, interface{}) Option
	- Memoize(bool) Option
	- Recover(bool) Option

See the godoc page of the generated parser for the test/predicates grammar
for an example documentation page of the exported API:
http://godoc.org/github.com/mna/pigeon/test/predicates.

Like the grammar used to generate the parser, the input text must be
UTF-8-encoded Unicode.

The start rule of the parser is the first rule in the PEG grammar used
to generate the parser. A call to any of the Parse* functions returns
the value generated by executing the grammar on the provided input text,
and an optional error.

Typically, the grammar should generate some kind of abstract syntax tree (AST),
but for simple grammars it may evaluate the result immediately, such as in
the examples/calculator example. There are no constraints imposed on the
author of the grammar, it can return whatever is needed.

Error reporting

When the parser returns a non-nil error, the error is always of type errList,
which is defined as a slice of errors ([]error). Each error in the list is
of type *parserError. This is a struct that has an "Inner" field that can be
used to access the original error.

So if a code block returns some well-known error like:
	{
		return nil, io.EOF
	}

The original error can be accessed this way:
	_, err := ParseFile("some_file")
	if err != nil {
		list := err.(errList)
		for _, err := range list {
			pe := err.(*parserError)
			if pe.Inner == io.EOF {
				// ...
			}
		}
	}

By defaut the parser will continue after an error is returned and will
cumulate all errors found during parsing. If the grammar reaches a point
where it shouldn't continue, a panic statement can be used to terminate
parsing. The panic will be caught at the top-level of the Parse* call
and will be converted into a *parserError like any error, and an errList
will still be returned to the caller.

The divide by zero error in the examples/calculator grammar leverages this
feature (no special code is needed to handle division by zero, if it
happens, the runtime panics and it is recovered and returned as a parsing
error).

Providing good error reporting in a parser is not a trivial task. Part
of it is provided by the pigeon tool, by offering features such as
filename, position, expected literals and rule name in the error message,
but an important part of good error reporting needs to be done by the grammar
author.

For example, many programming languages use double-quotes for string literals.
Usually, if the opening quote is found, the closing quote is expected, and if
none is found, there won't be any other rule that will match, there's no need
to backtrack and try other choices, an error should be added to the list
and the match should be consumed.

In order to do this, the grammar can look something like this:

	StringLiteral = '"' ValidStringChar* '"' {
		// this is the valid case, build string literal node
		// node = ...
		return node, nil
	} / '"'  ValidStringChar* !'"' {
		// invalid case, build a replacement string literal node or build a BadNode
		// node = ...
		return node, errors.New("string literal not terminated")
	}

This is just one example, but it illustrates the idea that error reporting
needs to be thought out when designing the grammar.

Because the above mentioned error types (errList and parserError) are not
exported, additional steps have to be taken, ff the generated parser is used as
library package in other packages (e.g. if the same parser is used in multiple
command line tools).
One possible implementation for exported errors (based on interfaces) and
customized error reporting (caret style formatting of the position, where
the parsing failed) is available in the json example and its command line tool:
http://godoc.org/github.com/mna/pigeon/examples/json

API stability

Generated parsers have user-provided code mixed with pigeon code
in the same package, so there is no package
boundary in the resulting code to prevent access to unexported symbols.
What is meant to be implementation
details in pigeon is also available to user code - which doesn't mean
it should be used.

For this reason, it is important to precisely define what is intended to be
the supported API of pigeon, the parts that will be stable
in future versions.

The "stability" of the version 1.0 API attempts to make a similar guarantee
as the Go 1 compatibility [5]. The following lists what part of the
current pigeon code falls under that guarantee (features may be added in
the future):

    - The pigeon command-line flags and arguments: those will not be removed
    and will maintain the same semantics.

    - The explicitly exported API generated by pigeon. See [6] for the
    documentation of this API on a generated parser.

    - The PEG syntax, as documented above.

    - The code blocks (except the initializer) will always be generated as
    methods on the *current type, and this type is guaranteed to have
    the fields pos (type position) and text (type []byte). There are no
    guarantees on other fields and methods of this type.

    - The position type will always have the fields line, col and offset,
    all defined as int. There are no guarantees on other fields and methods
    of this type.

    - The type of the error value returned by the Parse* functions, when
    not nil, will always be errList defined as a []error. There are no
    guarantees on methods of this type, other than the fact it implements the
    error interface.

    - Individual errors in the errList will always be of type *parserError,
    and this type is guaranteed to have an Inner field that contains the
    original error value. There are no guarantees on other fields and methods
    of this type.

The above guarantee is given to the version 1.0 (https://github.com/mna/pigeon/releases/tag/v1.0.0)
of pigeon, which has entered maintenance mode (bug fixes only). The current
master branch includes the development toward a future version 2.0, which
intends to further improve pigeon.
While the given API stability should be maintained as far as it makes sense,
breaking changes may be necessary to be able to improve pigeon.
The new version 2.0 API has not yet stabilized and therefore changes to the API
may occur at any time.

References:

    [5]: https://golang.org/doc/go1compat
    [6]: http://godoc.org/github.com/mna/pigeon/test/predicates

*/
package main
