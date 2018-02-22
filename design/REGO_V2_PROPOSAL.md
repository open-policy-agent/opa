Rego v2 - Proposal

Authors: Tristan Swadell, Tim Hinrichs, Torin Sandall
Last-Modified: 2018-02-22

# Goals

This document serves to facilitate collaborative development of the design of
a general-purpose policy language. _General-purpose_ means that the language
should be applicable to any domain, layer of the stack, or enforcement point.
Different implementations of the language runtime may be better suited to
different applications.

# Concepts

The user-experience for policy enforcement depends heavily on the policy
language and what concepts the user must understand to use that language. The
proposed concepts thus far are:

* **Rule** - An identifier with optional parameters that conditionally produces
             a decision.
    * May refer to other rules, constants, and functions.
    * Declared within modules.
    * May be overloaded.

* **Context** - Data provided at evaluation time or through calls to external
                data-sources.
    * Rules may declare a signature with the expected context.
    * External datasources may be a file, database, or API.

The proposed language has the following properties:

* **Side-effect free** and non-Turing complete.
* Rules, functions, and constants are declared within modules.
* Rules are evaluated by name or by module.
* Decisions are qualified by the module name where they are declared.
* Conflicts must be decided by a decision resolver or by the actor calling the
  engine.

# Rules

The rule declaration provides a named entry point for evaluation and
composition. The signature of the rule declares the context to be provided
upon evaluation, and the body is a collection of conditions and decisions.

Please note that the definition of `expr` and `literal` are taken from
the [Common Expression Language](http://github.com/google/cel-spec) (CEL):

```
rule_decl
    := 'rule' id assign_expr (if_expr else_expr*)?
    |  'rule' function_signature '{' statement+ '}'
    ;
const_decl
    :=  decorator? id assign_expr
    ;
assign_expr
    := '=' (expr | comprehension_expr)
    ;
function_decl
    := decorator? 'function' function_signature ('{' statement+ '}')?
    ;
function_signature
    : id '(' arg_list? ')'
    ;
arg_list
    := id (',' id)*
    ;
decorator:
    : '@' id
    ;
statement
    := condition_expr
    |  const_decl
    |  return_expr
    |  comprehension_expr
    ;
condition_expr
    := if_expr '{' statement+ '}'
    ;
if_expr
    := 'if' expr
    ;
else_expr
    := 'else' (expr | if_expr)
    ;
return_expr
    : 'return' (expr | comprehension_expr)
    ;
comprehension_expr
    := iter_expr
    |  '{' (expr '|')? iter_expr '}'
    |  '[' (expr '|')? iter_expr ']'
    ;
iter_expr
    := 'for' id (',' id)? 'in' expr if_expr? statement?
    |  'for' id (',' id)? 'in' expr if_expr? ('{' statement+ '}')?
    ;
```

Rules represents a decision and are declared distinctly from constants and
functions. Rules may be constant-like or function-like. Constant-like rules
yield decisions derived from module or system provided context. Function-like
rules declare a function signature that indicates the context required from the
caller. The body of the rule may contain any number of conditions, decisions,
and local declarations.

```
rule userSalary(resource, user) {
  match_result = resource.match('users/{target_user}/salary');
  return user == match_result.group.target_user;
}
```

Rules may be imported and leveraged within rule decisions. Note the inclusion
of the rule within a package and how this affects function identifier
resolution.

```
package acme;
import acme.hr;

rule readUserSalary(request, resource, user) {
  match_result = resource.match('users/{target_user}/salary');
  if (match_result.matches() && request.method == 'get') {
    target_user = match_result.groups.target_user
    return (hr.isManager(user, target_user)
         || hr.isAdmin(user)
         || user == target_user)
  }
  return false;
}

// outputs -> {'rule': 'acme.readUserSalary', 'result': bool}
```

Rules may have multiple return statements in order to enforce allow / deny
semantics (in the case of binary rules), or simply different variations on
an affirmative rule decision, such as whether to return a list of honey-pot
servers versus a valid list of servers.

```
rule readUserSalary(request, resource, user) {
  match_result = resource.match('users/{target_user}/salary')
  if (match_result.matches() && request.method == 'get') {
    target_user = match_result.groups.target_user
    // Deny requests outside of business hours.
    if (request.time.hour() < 9 || request.time.hour() > 17) {
       return false
    }
    return (hr.isManager(user, target_user)
         || hr.isAdmin(user)
         || user == target_user)
  }
  return false
}
```

Rules may be composed from other rules and functions. The example below
groups reading and listing salaries into a single decision.

```
// Rules may be composed.
rule viewSalary(request, resource, user) {
  return queryDepartmentSalaries(request, resource, user)
      || readUserSalary(request, resource, user);
}

rule queryDepartmentSalaries(request, resource, user) {
  match_result = resource.match('departments/{department}/salaries')
  if match_result.matches() && request.method == 'list' {
    department = match_result.groups.department;
    return !('user' in request.params.fields)
        && hr.department(department).manager == user;
  }
  return false;
}

// Evaluating of the 'acme' module produces:
// [{'rule': 'acme.viewSalary', 'result': bool},
//  {'rule': 'acme.viewUserSalary', 'result': bool},
//  {'rule': 'acme.queryDepartmentSalaries', 'result': bool}]
//
// Evaluation of just acme.viewSalary would yield only the first decision.
```

Rules may also be written in a constant style where overloading occurs on
the name. Developers may choose to write logically ORed statements together
or provide overloads as a means of augmenting the decision set that can be
attached to a rule.

```
// Constant-like rule which supports named evaluation.
//
// The authorized value is optionally assigned to the identifier 'allow'
// depending on the request.auth condition.
//
// Note, if the condition evaluates false, authenticated is not assigned and
// not included in the decision set.
rule authorized = allow if request.auth != null

// Overloads the authorized grant to also permit access to public resources.
//
// When the request is both authenticated and against a public resource, the
// decision set will include two authorized results.
//
// A conflict resolution strategy of 'anyAllow' would ensure a single result
// for the authorized decision. The default conflict resolution behavior could
// be to aggregate overloaded decisions if the type supports aggregation:
// e.g. sets, lists, boolean
// Conflict resolution requires further discussion.
rule authorized = allow if resource.name.contains('/public/')

// Rules may also be as a conditional assignment with if-else conditions rather
// than as overloads if the developer would like to control the overload
// behavior.
rule rateLimit = 100 if 'admin' in request.auth.claims
            else 50 if 'owner' in request.auth.claims
            else 10
```

## Conditions

Conditions are [Common Expression Language](https://github.com/google/cel-spec)
(CEL) expressions and evaluate to a boolean outcome. A condition may be used
to select applicable rules or to make an effect or trigger conditional.
Conditions may also be used to filter list and map entries within a `for-in`
expression.

Declarations within a condition are within its block scope and may be shadowed
by declarations in nested conditions. Once a declaration has been assigned, it
cannot be reassigned.

## Decisions

A rule represents a conditional decision which may depend on other rule
decisions. The types of overloaded rule declarations *should* agree. In the
case of multiple `rule`s being evaluated where the output types do not agree,
the decision is dynamically typed. Rule decisions are _maybe_ values, in the
sense that the result may be a valued type or undefined. This is a crucial
feature in support of rule overloading and evaluation with partial state.

The decision semantics when multiple `rule`s are evaluated depends on the
conflict resolution algorithm declared within the policy or on the caller.

Note: conflict resolution across `rule`s are as yet undefined, but will be
addressed in a future update to the proposal.

### Triggers

Triggers are not a separate concept, but rather a core consideration of how
the `rule` declarations are designed. Since each rule emits at most one
decision, this makes it feasible to write rules which go beyond request and
config validation, and apply to the conditional execution of additional
compute coordinated by the policy engine.

Conceptually, triggers align with the concepts of obligations and advice
introduced within [XACML](xacml.org). The difference between whether something
is an obligation or advice typically boils down to whether the action to
perform is synchronous or asynchronous. Synchronous obligations may include
preconditions, whereas async obligations would be considered promises. Advice,
on the other hand, should always be considered asynchronous and best-effort.

The following is an example of a rule that acts as a trigger to log request
behavior and check quota.

```
// This rule conditionally produces metadata indicating that a quota
// check should be performed. The evaluation engine will support the
// registration of decision handlers whose behavior will affect the
// overall request handling.
rule hasQuota(request) {
  if authenticated == allow {
    // Should return a payload flagged as an obligation, that can be
    // processed by a decision handler registered with the evaluator
    // for the <package-name>.hasQuota decision.
    return quota.check('perMethodPerUser',
        [{name: 'method', value: request.method},
         {name: 'user', value: request.auth.principal}])
  }
}

// The payload for a log statement could be as simple as a string.
rule log = logger.log(request.auth.principal
                      + " denied request on "
                      + resource.name)
           if authenticated != allow

// Module-based evaluation of these rules would output the following
// decisions:
// [{'rule': 'hasQuota',
//   'result': {
//     'type': 'obligation',
//     'handler': 'quota.check',
//     'metadata' : { 'metric': 'perMethodPerUser', args:[ ... ]}}},
//  {'rule': 'log',
//   'result': {
//     'type': 'promise',
//     'handler': 'logger.log',
//     'metadata': {'message': ...}}}]
```

# Tests

Being able to verify the correctness policy-related logic is of paramount
importance. As is the ability to pose ad hoc queries with partial state. To this
end we include `@test` as supported decorator and introduce the `with-as` clause
to assist with partial state bindings required for both adhoc queries and for
function mocking.

Note: the following is under review and not yet reflected in the grammar.

```
rule user_owned_action(auth, resource) {
  result = resource.matches('documents/{owner}/**')
  return (result.owner == auth.principal
       || resource.owner in user_groups(auth.principal));
}

@extern function user_groups(user);

function mock_user_groups(user) { … }

@test function group_check() {
   with user_groups as mock_get_group_users {
      assertTrue(user_owned_action({principal: 'me'}, 'documents/my-group'))
      assertFalse(user_owned_action({principal: 'me'}, 'documents/their-group'))
   }
}
```

# Context

Context is either supplied through arguments provided to the function, through
constant declarations within the module, or through external functions invoked
at evaluation time.

The following are all examples of how context may be provided:

```
package acme;
syntax = 'rego.v2';

// Functions bound at evaluation time.
@extern function resource();
@extern function query(document_name);

// Constant declaration based on external function calls.
ctx = {resource: resource().name,
       resource_owner: resource().owner};

// Allow user-owned reads, with user and request provided as a rule argument. 
rule allow_user_reads(user, request) {
  if (request.method in ['get', 'list']) {
      return (request.auth.claims.email == user
      || ctx.resource_owner == user
      || query("/documents/" + ctx.resource).created_by == user);
  }
  return false;
}
```

It will likely be common practice to provide @extern functions within their own
module like so:

```
package acme.db;
syntax = 'rego.v2';
@extern function resource();
@extern function query(document_name);
```

```
package acme;
import acme.db;
syntax = 'rego.v2';

// Constant declaration based on external function calls.
ctx = {'resource': db.resource().name,
       'resource_owner': db.resource().owner};

// Allow user-owned reads, with user and request provided as a rule argument. 
rule allow_user_reads(user, request) {
  if (request.method in ['get', 'list']) {
      return (request.auth.claims.email == user
      || ctx.resource_owner == user
      || db.query("/documents/" + ctx.resource).created_by == user);
  }
  return false;
}
```

The example below shows how `acme.db` provides a library of context and
functions for use with rules. The `@extern` decorator is equivalent to a
forward declaration, both to serve as documentation for what exists, but
also to be consumed during type-checking to ensure the system context and
function hooks are being used correctly within rules.

```
package acme.db;
syntax = 'rego.v2';
@extern request;
@extern function resource();
@extern function query(document_name);

input = {
  'method': request.method,
  'user': request.auth.claims.email,
  'resource': resource()
}

permission = {
  'read': input.method in ['get', 'list'];
  'write': input.method in ['create', 'update', 'delete'];
}

function resourceMatch(pattern) {
  return resource().name.match(pattern).groups;
}
```

```
package acme;
import acme.db;
syntax = 'rego.v2';

// Allow user-owned reads, with context information provided by functions and
// constants provided by acme.db in the form of module or extern declarations.
rule authorized() {
  target_user = db.resourceMatch('users/{target_user}/salary').target_user;
  return db.permission.read
    && (db.input.user == target_user
    || db.input.resource.owner == target_user
    || db.query("/documents/" + input.resource.name).created_by == target_user);
}
```

# Language

Rego v2 is a series of extensions to the Common Expression Language (CEL) with
the aim of clarifying the flow of execution from Rego v1 while preserving its
features and incorporating the non-Turing complete, partial evaluation
semantics of CEL.

## Grammar

```
module
    := package? import* decls
    ;
package
    := 'package' qualified_id ';'
    ;
import
    := 'import' qualified_id ('as' id)? ';'
    ;
decls
    := (rule_decl | function_decl | const_decl)*
    ;
rule_decl
    := 'rule' id assign_expr (if_expr else_expr*)?
    |  'rule' function_signature '{' statement+ '}'
    ;
const_decl
    :=  id assign_expr
    |   decorator id
    ;
assign_expr
    := '=' (expr | comprehension_expr)
    ;
function_decl
    := decorator? 'function' function_signature ('{' statement+ '}')?
    ;
function_signature
    : id '(' arg_list? ')'
    ;
arg_list
    := id (',' id)*
    ;
decorator:
    : '@' id
    ;
statement
    := condition_expr
    |  const_decl
    |  return_expr
    |  comprehension_expr
    |  with_block
    ;
condition_expr
    := if_expr '{' statement+ '}'
    ;
if_expr
    := 'if' expr
    ;
else_expr
    := 'else' (expr | if_expr)
    ;
return_expr
    : 'return' (expr | comprehension_expr)
    ;
comprehension_expr
    := iter_expr
    |  '{' (expr '|')? iter_expr '}'
    |  '[' (expr '|')? iter_expr ']'
    ;
iter_expr
    := 'for' id (',' id)? 'in' expr if_expr? statement?
    |  'for' id (',' id)? 'in' expr if_expr? ('{' statement+ '}')?
    ;
with_block
    : 'with' qualified_id 'as' id '{' statement+ '}'
    ;
expr
    := or_expr ('?' or_expr ':' expr)?
    ;
or_expr
    := and_expr ('||' and_expr)*
    ;
and_expr
    := relation_expr ('&&' relation_expr)*
    ;
relation_expr
    := calc_expr
    |  relation_expr ('<'|'<='|'>='|'>'|'=='|'!='|'in') relation_expr
    ;
calc_expr
    := unary_expr
    |  calc_expr ('*'|'/'|'%') calc_expr
    |  calc_expr ('+'|'-') calc_expr
    ;
unary_expr
    := member_expr
    |  '!'+ member_expr
    |  '-'+ member_expr
    ;
member_expr
    := primary_expr
    |  member_expr '.' id
    |  member_expr '.' id '(' expr_list? ')'
    |  member_expr '[' expr ']'
    |  qualified_id '{' field_inits? '}'
    ;
primary_expr
    := '.'? id
    |  '.'? id '(' expr_list? ')'
    |  '(' expr ')'
    |  '[' expr_list? ']'
    |  '{' map_inits? '}'
    |  literal
    ;
expr_list
    := expr (',' expr)*
    ;
field_inits
    := id ':' expr (',' id ':' expr)*
    ;
map_inits
    := expr ':' expr (',' expr : 'expr')*
    ;
qualified_id
    := '.'? id ('.' id)*
    ;
```

Note: CEL and Rego are gradually typed languages. Although the proposal does
not describe developer-defined type designations on constants and functions,
this may be introduced in the future.

## Constants

Constants are identifiers associated with an expression. They are useful for
clarifying the logical relationship between components of a rule decision.
Constants referenced within expression will evaluate in the same manner as
though they were written inline into the statement.

## Functions

Functions are simply collections of logical statements. Functions decorated
with `@extern` must only declare a signature as the implementation is provided
by the calling environment. Functions decorated with `@test` may incorporate
`with` blocks and cannot be directly or indirectly referenced by `rule`
declarations.

Functions are idempotent. Given the same input, the functions must return
the same output. Idempotency must hold for both local and extern functions.
At present functions do not support overloads, though this may change in
future iterations of this proposal.

The general form of a function is as follows:

```
function_decl
    := decorator? 'function' id '(' arg_list? ')' ('{' statement+ '}')?
    ;
decorator
    := '@' id
    ;
```

The set of supported decorators is limited to `@extern` and `@test`. When a
function is annotated with `@extern`, it must be supplied at runtime as part of
the rule evaluation context. For example:

```
// Environment must bind a handler for 'db.get' function calls from the policy.
package db;
@extern function get(document_name);
```

## Comprehensions

The one key difference between a Rego v2 expression and a CEL expression is
that Rego v2 has an explicit syntax for list comprehensions rather than use the
optional CEL macros. This syntax simplifies quantifiers as well as mapping and
filtering while permitting the introduction of specialized reducing functions.
It is recommended, but not required that comprehensions be performed within
local functions rather inline within a rule, but this is not a requirement.

The general form of a comprehension is as follows:

```
comprehension_expr
    := iter_expr
    |  '{' (expr '|')? iter_expr '}'  // set, map comprehension
    |  '[' (expr '|')? iter_expr ']'  // list comprehension
    ;
iter_expr
    := 'for' id (',' id)? 'in' expr iter_op_expr? statement?
    |  'for' id (',' id)? 'in' expr iter_op_expr? ('{' statement+ '}')?
    ;
iter_op_expr
    := if_expr
    |  'all' expr
    |  'any' expr
    ;
filter_expr
    := 'if' expr
    ;
```

A set comprehension differs from a list comprehension only in the sense that
the entries within the set are unique. Set comprehensions are also used for
constructing maps and complex objects where the uniqueness constraint generally
holds true. Within maps, colliding keys are overwritten. Within a
comprehension, colliding keys values are merged if the value is declared as a
list.

### Filter

The `for-in` expression may be filtered using a condition. The result of the
filtering, absent of other syntax, will be the same as the range type of the
`for-in` expression. If the for-in range is a list, the result type will be a
list. The same is true for maps, structs, and messages.

```
for u in users if u.clearance in ['secret', 'top secret'] // list of users
```

Filters may be chained as well.

```
{ user: [doc] |
// Iteration variables may be referenced within the transform statement
for user in users if user.clearance in ['secret', 'top_secret']
// The statement after a for-in is implicitly within its block scope.
// Curly braces may be used to explicitly denote the block.
for doc in documents if doc.clearance <= user.clearance }
```

### Quantify

The quantifiers rely on filtering and simply test the `size()` of the filter
result. The result type of the for-in is determined by the surrounding braces.

```
// exists one
[ for e in list if e < 10 ].size() == 1

// exists at least two unique values, note the `{}` surrounding the `for-in`
// indicates this is the construction of a set from a list.
{ for e in list if e.startsWith('t') }.size() > 2
```

There are also two special operators which can be used to make qualitative
inferences about the elements within an aggregate type, `all` and `any`. The
primary difference between these operators and the filter expressions (or
even standard comprehensions) is that they are accumulator functions with
the same semantics as the logical `&&` and `||`, meaning these operators
can absorb errors.

```
// Any element exists in this list
for e in list any e != 'bad_candidate'

// All elements in this list must be good candidates
for e in list all e == 'good_candidate'
```

The `any` and `all` yield boolean outcomes and are thus specialized reducing
functions capable of absorbing errors in the same manner as hand-rolled code.

### Map

It is very common for developers to massage data from a variety of sources in
order to compose the desired format best suited for the task at hand.

```
// Set of unique image names
{ image.name | for release_name, image in release_images }

// Map of image name to release names
// Collisions on the image.name will overwrite the existing value.
// Note: Consider adding syntactic sugar to deal with object construction
// during list comprehensions.
{ image.name : release_name | for release_name, image in release_images }

// List (of pairs) comprehension.
// There is an implicit single-statement block after a for-in if no
// curly braces are present.
[ [user, file] |
  for file in files
  for user in file.viewers + [file.owner]
  if user.matches("@{domain}.{tld}$").groups.domain in ["styra", "google"]
]

// Equivalent to the above, but with a set of unique user, file tuples and
// explicit block syntax.
{ [user, file] |
  for file in files {
    for user in file.viewers + [file.owner]
    if user.matches("@{domain}.{tld}$").groups.domain in ["styra", "google"] ]
  }
}
```

### Reduce

Rather than introduce a special syntax for reduction, Rego v2 provides the
helper functions: `sum()`, `flatten()`, and `join()`. Additional functions
will be added over time, but these helpers represent the most common use
cases of reduction.

## Modules

A module represents a collection of functions and constants. Modules are in
the root package unless otherwise indicated by a package declaration. Multiple
modules may share the same package. Functions in modules within the same
package are accessible within other modules without specifying the qualified
function name, e.g. `module.func()`.

Importing a module makes all symbols within that module accessible by either
the fully qualified module name, or the simple module name (the last fragment
of a qualified package identifier).


