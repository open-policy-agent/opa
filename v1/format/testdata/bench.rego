# METADATA
# description: |
#   the 'ast' package provides the base functionality for working
#   with OPA's AST, more recently in the form of RoAST
package regal.ast

import data.regal.config
import data.regal.util

# METADATA
# description: set of Rego's scalar type
scalar_types := {"boolean", "null", "number", "string"}

# METADATA
# description: set containing names of all built-in functions counting as operators
operators := {
	"and",
	"assign",
	"div",
	"eq",
	"equal",
	"gt",
	"gte",
	"internal.member_2",
	"internal.member_3",
	"lt",
	"lte",
	"minus",
	"mul",
	"neq",
	"or",
	"plus",
	"rem",
}

# METADATA
# description: |
#   returns true if provided term is either a scalar or a collection of ground values
# scope: document
is_constant(term) if term.type in scalar_types # regal ignore:external-reference

is_constant(term) if {
	term.type in {"array", "object"}
	not has_term_var(term.value)
}

# METADATA
# description: true if provided term represents a wildcard (`_`) variable
is_wildcard(term) if {
	term.type == "var"
	startswith(term.value, "$")
}

default builtin_names := set()

# METADATA
# description: set containing the name of all built-in functions (given the active capabilities)
# scope: document
builtin_names := object.keys(config.capabilities.builtins)

# METADATA
# description: |
#   set containing the namespaces of all built-in functions (given the active capabilities),
#   like "http" in `http.send` or "sum" in `sum``
builtin_namespaces contains namespace if {
	some name in builtin_names
	namespace := split(name, ".")[0]
}

# METADATA
# description: |
#   provides the package path values (strings) as an array starting _from_ "data":
#   package foo.bar -> ["foo", "bar"]
package_path := [path.value |
	some i, path in input["package"].path
	i > 0
]

# METADATA
# description: |
#   provide the package name / path as originally declared in the
#   input policy, so "package foo.bar" would return "foo.bar"
package_name := concat(".", package_path)

# METADATA
# description: provides all static string values from ref
named_refs(ref) := [term |
	some i, term in ref
	_is_name(term, i)
]

_is_name(term, 0) if term.type == "var"

_is_name(term, pos) if {
	pos > 0
	term.type == "string"
}

# METADATA
# description: all the rules (excluding functions) in the input AST
rules := [rule |
	some rule in input.rules
	not rule.head.args
]

# METADATA
# description: all the test rules in the input AST
tests := [rule |
	some rule in input.rules
	not rule.head.args

	startswith(ref_to_string(rule.head.ref), "test_")
]

# METADATA
# description: all the functions declared in the input AST
functions := [rule |
	some rule in input.rules
	rule.head.args
]

# METADATA
# description: |
#   all rules and functions in the input AST not denoted as private, i.e. excluding
#   any rule/function with a `_` prefix. it's not unthinkable that more ways to denote
#   private rules (or even packages), so using this rule should be preferred over
#   manually checking for this using the rule ref
public_rules_and_functions := [rule |
	some rule in input.rules

	count([part |
		some i, part in rule.head.ref

		_private_rule(i, part)
	]) == 0
]

_private_rule(0, part) if startswith(part.value, "_")

_private_rule(i, part) if {
	i > 0
	part.type == "string"
	startswith(part.value, "_")
}

# METADATA
# description: a list of the argument names for the given rule (if function)
function_arg_names(rule) := [arg.value | some arg in rule.head.args]

# METADATA
# description: all the rule and function names in the input AST
rule_and_function_names contains ref_to_string(rule.head.ref) if some rule in input.rules

# METADATA
# description: all identifiers in the input AST (rule and function names, plus imported names)
identifiers := rule_and_function_names | imported_identifiers

# METADATA
# description: all rule names in the input AST (excluding functions)
rule_names contains ref_to_string(rule.head.ref) if some rule in rules

# METADATA
# description: |
#   determine if var in var (e.g. `x` in `input[x]`) is used as input or output
# scope: document
is_output_var(rule, var) if {
	# test the cheap and common case first, and 'else' only when it's not
	is_wildcard(var)
} else if {
	not var.value in (rule_names | imported_identifiers) # regal ignore:external-reference

	num_above := count([1 |
		some above in find_vars_in_local_scope(rule, var.location)
		above.value == var.value
	])
	num_some := count([1 |
		some name in find_some_decl_names_in_scope(rule, var.location)
		name == var.value
	])

	# only the first ref variable in scope can be an output! meaning that:
	# allow if {
	#     some x
	#     input[x]    # <--- output
	#     data.bar[x] # <--- input
	# }
	num_above - num_some == 0
}

# METADATA
# description: as the name implies, answers whether provided value is a ref
# scope: document
is_ref(value) if value.type == "ref"

is_ref(value) if value[0].type == "ref"

# METADATA
# description: |
#   returns an array of all rule indices, as strings. this will be needed until
#   https://github.com/open-policy-agent/opa/issues/6736 is fixed
rule_index_strings := [s |
	some i, _ in _rules
	s := sprintf("%d", [i])
]

# METADATA
# description: |
#   a map containing all function calls (built-in and custom) in the input AST
#   keyed by rule index
function_calls[rule_index] contains call if {
	some rule_index in rule_index_strings
	some ref in found.refs[rule_index]

	name := ref_to_string(ref[0].value)
	args := [arg |
		some i, arg in array.slice(ref, 1, 100)

		not _exclude_arg(name, i, arg)
	]

	call := {
		"name": ref_to_string(ref[0].value),
		"location": ref[0].location,
		"args": args,
	}
}

# these will be aggregated as calls anyway, so let's try and keep this flat
_exclude_arg(_, _, arg) if arg.type == "call"

# first "arg" of assign is the variable to assign to.. special case we simply
# ignore here, as it's covered elsewhere
_exclude_arg("assign", 0, _)

# METADATA
# description: returns the "path" string of any given ref value
ref_to_string(ref) := concat("", [_ref_part_to_string(i, part) | some i, part in ref])

_ref_part_to_string(0, part) := part.value

_ref_part_to_string(i, part) := _format_part(part) if i > 0

_format_part(part) := sprintf(".%s", [part.value]) if {
	part.type == "string"
	regex.match(`^[a-zA-Z_][a-zA-Z1-9_]*$`, part.value)
} else := sprintf(`["%v"]`, [part.value]) if {
	part.type == "string"
} else := sprintf(`[%v]`, [part.value])

# METADATA
# description: |
#   returns the string representation of a ref up until its first
#   non-static (i.e. variable) value, if any:
#   foo.bar -> foo.bar
#   foo.bar[baz] -> foo.bar
ref_static_to_string(ref) := str if {
	rs := ref_to_string(ref)
	str := _trim_from_var(rs, regex.find_n(`\[[^"]`, rs, 1))
}

_trim_from_var(ref_str, vars) := ref_str if {
	count(vars) == 0
} else := substring(ref_str, 0, indexof(ref_str, vars[0]))

# METADATA
# description: true if ref contains only static parts
static_ref(ref) if not _non_static_ref(ref)

# optimized inverse of static_ref benefitting from early exit
# 128 is used only as a reasonable (well...) upper limit for a ref, but the
# slice will be capped at the length of the ref anyway (avoids count)
_non_static_ref(ref) if array.slice(ref.value, 1, 128)[_].type in {"var", "ref"}

# METADATA
# description: provides a set of names of all built-in functions called in the input policy
builtin_functions_called contains name if {
	name := function_calls[_][_].name
	name in builtin_names
}

# METADATA
# description: |
#   Returns custom functions declared in input policy in the same format as builtin capabilities
function_decls(rules) := {rule_name: decl |
	# regal ignore:external-reference
	some rule in functions

	rule_name := ref_to_string(rule.head.ref)

	# ensure we only get one set of args, or we'll have a conflict
	args := [[item |
		some arg in rule.head.args
		item := {"type": "any"}
	] |
		some rule in rules
		ref_to_string(rule.head.ref) == rule_name
	][0]

	decl := {"decl": {"args": args, "result": {"type": "any"}}}
}

# METADATA
# description: returns the args for function past the expected number of args
function_ret_args(fn_name, terms) := array.slice(terms, count(all_functions[fn_name].decl.args) + 1, count(terms))

# METADATA
# description: true if last argument of function is a return assignment
function_ret_in_args(fn_name, terms) if {
	# special case: print does not have a last argument as it's variadic
	fn_name != "print"

	rest := array.slice(terms, 1, count(terms))

	# for now, bail out of nested calls
	not "call" in {term.type | some term in rest}

	count(rest) > count(all_functions[fn_name].decl.args)
}

# METADATA
# description: answers if provided rule is implicitly assigned boolean true, i.e. allow { .. } or not
# scope: document
implicit_boolean_assignment(rule) if {
	# note the missing location attribute here, which is how we distinguish
	# between implicit and explicit assignments
	rule.head.value == {"type": "boolean", "value": true}
}

# or sometimes, like this...
implicit_boolean_assignment(rule) if rule.head.value.location == rule.head.location

implicit_boolean_assignment(rule) if util.to_location_object(rule.head.value.location).col == 1

# METADATA
# description: |
#   object containing all available built-in and custom functions in the
#   scope of the input AST, keyed by function name
all_functions := object.union(config.capabilities.builtins, function_decls(input.rules))

# METADATA
# description: |
#   set containing all available built-in and custom function names in the
#   scope of the input AST
all_function_names := object.keys(all_functions)

# METADATA
# description: set containing all negated expressions in input AST
negated_expressions[rule_index] contains value if {
	some i, rule in _rules

	# converting to string until https://github.com/open-policy-agent/opa/issues/6736 is fixed
	rule_index := sprintf("%d", [i])

	walk(rule, [_, value])

	value.negated
}

# METADATA
# description: |
#   true if rule head contains no identifier, but is a chained rule body immediately following the previous one:
#   foo {
#       input.bar
#   } {	# <-- chained rule body
#       input.baz
#   }
is_chained_rule_body(rule, lines) if {
	head_loc := util.to_location_object(rule.head.location)

	row_text := lines[head_loc.row - 1]
	col_text := substring(row_text, head_loc.col - 1, -1)

	startswith(col_text, "{")
}

# METADATA
# description: answers wether variable of `name` is found anywhere in provided rule `head`
# scope: document
var_in_head(head, name) if {
	head.value.value == name
} else if {
	head.key.value == name
} else if {
	some var in find_term_vars(head.value.value)
	var.value == name
} else if {
	some var in find_term_vars(head.key.value)
	var.value == name
} else if {
	some i, var in head.ref
	i > 0
	var.value == name
}

# METADATA
# description: |
#   true if var of `name` is referenced in any `calls` (likely,
#   `ast.function_calls`) in the rule of given `rule_index`
# scope: document
var_in_call(calls, rule_index, name) if _var_in_arg(calls[rule_index][_].args[_], name)

_var_in_arg(arg, name) if {
	arg.type == "var"
	arg.value == name
}

_var_in_arg(arg, name) if {
	arg.type in {"array", "object", "set"}

	some var in find_term_vars(arg)

	var.value == name
}

# METADATA
# description: answers wether provided expression is an assignment (using `:=`)
is_assignment(expr) if {
	expr.terms[0].type == "ref"
	expr.terms[0].value[0].type == "var"
	expr.terms[0].value[0].value == "assign"
}

# METADATA
# description: returns the terms in an assignment (`:=`) expression, or undefined if not assignment
assignment_terms(expr) := [expr.terms[1], expr.terms[2]] if is_assignment(expr)

# METADATA
# description: |
#   For a given rule head name, this rule contains a list of locations where
#   there is a rule head with that name.
rule_head_locations[name] contains {"row": loc.row, "col": loc.col} if {
	some rule in input.rules

	name := concat(".", [
		"data",
		package_name,
		ref_static_to_string(rule.head.ref),
	])

	loc := util.to_location_object(rule.head.location)
}

_find_nested_vars(obj) := [value |
	walk(obj, [_, value])
	value.type == "var"
	indexof(value.value, "$") == -1
]

# simple assignment, i.e. `x := 100` returns `x`
# always returns a single var, but wrapped in an
# array for consistency
_find_assign_vars(value) := var if {
	value[1].type == "var"
	var := [value[1]]
}

# 'destructuring' array assignment, i.e.
# [a, b, c] := [1, 2, 3]
# or
# {a: b} := {"foo": "bar"}
_find_assign_vars(value) := vars if {
	value[1].type in {"array", "object"}
	vars := _find_nested_vars(value[1])
}

# var declared via `some`, i.e. `some x` or `some x, y`
_find_some_decl_vars(value) := [v |
	some v in value
	v.type == "var"
]

# single var declared via `some in`, i.e. `some x in y`
_find_some_in_decl_vars(value) := vars if {
	arr := value[0].value
	count(arr) == 3

	vars := _find_nested_vars(arr[1])
}

# two vars declared via `some in`, i.e. `some x, y in z`
_find_some_in_decl_vars(value) := vars if {
	arr := value[0].value
	count(arr) == 4

	vars := [v |
		some i in [1, 2]
		some v in _find_nested_vars(arr[i])
	]
}

# METADATA
# description: |
#   find vars like input[x].foo[y] where x and y are vars
#   note: value.type == "ref" check must have been done before calling this function
find_ref_vars(value) := [var |
	some i, var in value.value

	i > 0
	var.type == "var"
]

# one or two vars declared via `every`, i.e. `every x in y {}`
# or `every`, i.e. `every x, y in y {}`
_find_every_vars(value) := vars if {
	key_var := [value.key |
		value.key.type == "var"
		indexof(value.key.value, "$") == -1
	]
	val_var := [value.value |
		value.value.type == "var"
		indexof(value.value.value, "$") == -1
	]

	vars := array.concat(key_var, val_var)
}

# METADATA
# description: |
#   traverses all nodes in provided terms (using `walk`), and returns an array with
#   all variables declared in terms, i,e [x, y] or {x: y}, etc.
find_term_vars(terms) := [term |
	walk(terms, [_, term])

	term.type == "var"
]

# METADATA
# description: |
#   traverses all nodes in provided terms (using `walk`), and returns true if any variable
#   is found in terms, with early exit (as opposed to find_term_vars)
has_term_var(terms) if {
	walk(terms, [_, term])

	term.type == "var"
}

_find_vars(value, last) := {"term": find_term_vars(function_ret_args(fn_name, value))} if {
	last == "terms"
	value[0].type == "ref"
	value[0].value[0].type == "var"
	value[0].value[0].value != "assign"

	fn_name := ref_to_string(value[0].value)

	not contains(fn_name, "$")
	fn_name in all_function_names # regal ignore:external-reference
	function_ret_in_args(fn_name, value)
}

# `=` isn't necessarily assignment, and only considering the variable on the
# left-hand side is equally dubious, but we'll treat `x = 1` as `x := 1` for
# the purpose of this function until we have a more robust way of dealing with
# unification
_find_vars(value, last) := {"assign": _find_assign_vars(value)} if {
	last == "terms"
	value[0].type == "ref"
	value[0].value[0].type == "var"
	value[0].value[0].value in {"assign", "eq"}
}

_find_vars(value, last) := {"somein": _find_some_in_decl_vars(value)} if {
	last == "symbols"
	value[0].type == "call"
}

_find_vars(value, last) := {"some": _find_some_decl_vars(value)} if {
	last == "symbols"
	value[0].type != "call"
}

_find_vars(value, last) := {"every": _find_every_vars(value)} if {
	last == "terms"
	value.domain
}

_find_vars(value, last) := {"args": arg_vars} if {
	last == "args"

	arg_vars := [arg |
		some arg in value
		arg.type == "var"
	]

	count(arg_vars) > 0
}

_rule_index(rule) := sprintf("%d", [i]) if {
	some i, r in _rules # regal ignore:external-reference
	r == rule
}

# METADATA
# description: |
#   traverses all nodes under provided node (using `walk`), and returns an array with
#   all variables declared via assignment (:=), `some`, `every` and in comprehensions
#   DEPRECATED: uses ast.found.vars instead
find_vars(node) := array.concat(
	[var |
		walk(node, [path, value])

		last := regal.last(path)
		last in {"terms", "symbols", "args"}

		var := _find_vars(value, last)[_][_]
	],
	[var |
		walk(node, [_, value])

		value.type == "ref"

		some x, var in value.value
		x > 0
		var.type == "var"
	],
)

# hack to work around the different input models of linting vs. the lsp package.. we
# should probably consider something more robust
_rules := input.rules

_rules := data.workspace.parsed[input.regal.file.uri].rules if not input.rules

# METADATA:
# description: |
#   object containing all variables found in the input AST, keyed first by the index of
#   the rule where the variables were found (as a numeric string), and then the context
#   of the variable, which will be one of:
#   - term
#   - assign
#   - every
#   - some
#   - somein
#   - ref
found.vars[rule_index][context] contains var if {
	some i, rule in _rules

	# converting to string until https://github.com/open-policy-agent/opa/issues/6736 is fixed
	rule_index := sprintf("%d", [i])

	walk(rule, [path, value])

	last := regal.last(path)
	last in {"terms", "symbols", "args"}

	some context, vars in _find_vars(value, last)
	some var in vars
}

found.vars[rule_index].ref contains var if {
	some i, rule in _rules

	# converting to string until https://github.com/open-policy-agent/opa/issues/6736 is fixed
	rule_index := sprintf("%d", [i])

	walk(rule, [_, value])

	value.type == "ref"

	some x, var in value.value
	x > 0
	var.type == "var"
}

# METADATA
# description: all refs found in module
# scope: document
found.refs[rule_index] contains value if {
	some i, rule in _rules

	# converting to string until https://github.com/open-policy-agent/opa/issues/6736 is fixed
	rule_index := sprintf("%d", [i])

	walk(rule, [_, value])

	value.type == "ref"
}

found.refs[rule_index] contains value if {
	some i, rule in _rules

	# converting to string until https://github.com/open-policy-agent/opa/issues/6736 is fixed
	rule_index := sprintf("%d", [i])

	walk(rule, [_, value])

	value[0].type == "ref"
}

# METADATA
# description: all symbols found in module
found.symbols[rule_index] contains value.symbols if {
	some i, rule in _rules

	# converting to string until https://github.com/open-policy-agent/opa/issues/6736 is fixed
	rule_index := sprintf("%d", [i])

	walk(rule, [_, value])
}

# METADATA
# description: all comprehensions found in module
found.comprehensions[rule_index] contains value if {
	some i, rule in _rules

	# converting to string until https://github.com/open-policy-agent/opa/issues/6736 is fixed
	rule_index := sprintf("%d", [i])

	walk(rule, [_, value])

	value.type in {"arraycomprehension", "objectcomprehension", "setcomprehension"}
}

# METADATA
# description: |
#   finds all vars declared in `rule` *before* the `location` provided
#   note: this isn't 100% accurate, as it doesn't take into account `=`
#   assignments / unification, but it's likely good enough since other rules
#   recommend against those
find_vars_in_local_scope(rule, location) := [var |
	var := found.vars[_rule_index(rule)][_][_] # regal ignore:external-reference

	not is_wildcard(var)
	_before_location(rule, var, util.to_location_object(location))
]

_end_location(location) := end if {
	loc := util.to_location_object(location)
	lines := split(loc.text, "\n")
	end := {
		"row": (loc.row + count(lines)) - 1,
		"col": loc.col + count(regal.last(lines)),
	}
}

# special case — the value location of the rule head "sees"
# all local variables declared in the rule body
_before_location(rule, _, location) if {
	loc := util.to_location_object(location)

	value_start := util.to_location_object(rule.head.value.location)

	loc.row >= value_start.row
	loc.col >= value_start.col

	value_end := _end_location(util.to_location_object(rule.head.value.location))

	loc.row <= value_end.row
	loc.col <= value_end.col
}

_before_location(_, var, location) if {
	util.to_location_object(var.location).row < util.to_location_object(location).row
}

_before_location(_, var, location) if {
	var_loc := util.to_location_object(var.location)
	loc := util.to_location_object(location)

	var_loc.row == loc.row
	var_loc.col < loc.col
}

# METADATA
# description: find *only* names in the local scope, and not e.g. rule names
find_names_in_local_scope(rule, location) := names if {
	fn_arg_names := _function_arg_names(rule)
	var_names := {var.value | some var in find_vars_in_local_scope(rule, util.to_location_object(location))}

	names := fn_arg_names | var_names
}

_function_arg_names(rule) := {arg.value |
	some arg in rule.head.args
	arg.type == "var"
}

# METADATA
# description: |
#   similar to `find_vars_in_local_scope`, but returns all variable names in scope
#   of the given location *and* the rule names present in the scope (i.e. module)
find_names_in_scope(rule, location) := names if {
	locals := find_names_in_local_scope(rule, util.to_location_object(location))

	# parens below added by opa-fmt :)
	names := (rule_names | imported_identifiers) | locals
}

# METADATA
# description: |
#   find all variables declared via `some` declarations (and *not* `some .. in`)
#   in the scope of the given location
find_some_decl_names_in_scope(rule, location) := {some_var.value |
	some some_var in found.vars[_rule_index(rule)]["some"] # regal ignore:external-reference
	_before_location(rule, some_var, location)
}

# METADATA
# description: all expressions in module
exprs[rule_index][expr_index] := expr if {
	some rule_index, rule in input.rules
	some expr_index, expr in rule.body
}
