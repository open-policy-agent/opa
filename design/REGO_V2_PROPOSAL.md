Rego v2 - Proposal

Authors: Tristan Swadell, Tim Hinrichs, Torin Sandall 
Last-Modified: 2017-12-15

# Goals

This document serves to facilitate collaborative development of the design of
a general-purpose policy language. _General-purpose_ means that the language
should be applicable to any domain, any layer of the stack, and any enforcement
point. Different implementations of the language runtime may be better suited
to different applications.

# Concepts

The user-experience for policy enforcement depends heavily on the policy
language and what concepts the user must understand to use that language. The
proposed concepts thus far are:

* **Rule**
    * Functions or constants that provide a typed decision, e.g. boolean,
      list, map, etc.
    * Determine which sub-rules are relevant and their computational complexity.
    * May be packaged into modules to support logic reuse and the construction
      of libraries for specific domains (K8s, service meshes, cloud APIs).

* **Trigger**
    * Functions that produce side-effects which run after rules.
    * May inspect rule context.
    * Side-effects may be one of:
        * Obligation - guaranteed to execute if the request succeeds, may be
          synchronous (precondition) or asynchronous (promise).`
        * Advice - best-effect asynchronous execution on request success.

* **Context** 
    * Provided at evaluation time, or through calls to external data-sources. 
    * Datasources may be either JSON, Protobuf, Relational, or Graph databases.

The proposed language has the following properties:

* Rules are packaged into modules
* The absence of qualified package name indicates the default package.
* Rules are evaluated by name or using the `main()` entry point if no name is
  specified.
* Conditions determine applicable decisions.
* Conditions may be **hierarchical**.
* Conditions may invoke other rules.
* Decisions are **evaluated in order, first decision wins**. 
* A rule must always make a decision.
* Conditions, decisions, local declarations, and triggers may invoke functions.
* Functions are **side-effect free**.
* Triggers run after rules and may be cross-cutting.

# Rules

The rule declaration provides a named entry point for evaluation and
composition. The signature of the rule declares the context to be provided
upon evaluation, and the body is a collection of conditions and decisions.

The general form of rule is effectively a decorated function or constant with
the relevant portion of the grammar defined in the Language section presented
below. Please note that the definition of `expr` and `literal` are taken from
the [Common Expression Language](http://github.com/google/cel-spec) (CEL):

```
function_decl 
    := decorator? 'function' id '(' arg_list? ')' ('{' statement+ '}')?
    ;
decorator 
    := '@' id decorator_arg_list?
    ;
decorator_arg_list
    := '(' id '=' literal (',' id '=' literal)* ')'
    ;
arg_list 
    := id (',' id)*
    ;
statement 
    := condition_expr 
    |  const_decl 
    |  return_expr 
    |  comprehension_expr
    |  block_expr
    ;
condition_expr
    := filter_expr '{' statement+ '}'
    ;
filter_expr
    := 'if' expr
    ;
const_decl 
    :=  decorator? id assign_expr? ';'
    ;
assign_expr
    := '=' (expr | comprehension_expr)
    ;
return_expr
    : 'return' (expr | comprehension_expr) ';'
    ;
comprehension_expr
    := iter_expr
    |  '{' (expr '|')? iter_expr '}'
    |  '[' (expr '|')? iter_expr ']'
    ;
iter_expr
    := 'for' id (',' id)? 'in' expr filter_expr? statement?;
    |  'for' id (',' id)? 'in' expr filter_expr? ('{' statement+ '}')?
    ;
block_expr
    := decorator? '{' statement+ '}';
```

Rules represents a decision and are marked with the `@rule` decorator. A rule
may either be a constant or a function. A rule may be a constant when the
decision may be derived entirely from module or system provided context. For
function rules, the signature of the function indicates the context required
from the caller. The body of the rule may contain any number of conditions,
decisions, and local declarations. The decorator indicates that the function
or value is intended to be used for decisions rather than simply being an
intermediate piece of logic.

```
@rule function userSalary(resource, user) {
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

@rule function readUserSalary(request, resource, user) {
  match_result = resource.match('users/{target_user}/salary');
  if (match_result.matches() && request.method == 'get') {   
    target_user = match_result.groups.target_user;
    return (hr.isManager(user, target_user)
         || hr.isAdmin(user)
         || user == target_user);
  }
  return false;
}
```

Rules may have multiple return statements in order to enforce allow / deny
semantics (in the case of binary rules), or simply different variations on
an affirmative rule decision, such as whether to return a list of honey-pot
servers versus a valid list of servers.

```
@rule function readUserSalary(request, resource, user) {
  match_result = resource.match('users/{target_user}/salary'); 
  if (match_result.matches() && request.method == 'get') {   
    target_user = match_result.groups.target_user;
    // Deny requests outside of business hours.
    if (request.time.hour() < 9 || request.time.hour() > 17) {
       return false;
    }
    return (hr.isManager(user, target_user)
         || hr.isAdmin(user)
         || user == target_user);
  }
  return false;
}
```

Rules may be composed. The example below indicates how reading and listing a
salary are lumped into a single decision.

```
// Rules may be composed.
@rule function viewSalary(request, resource, user) {
  return queryDepartmentSalaries(request, resource, user)
      || readUserSalary(request, resource, user);
}

@rule function queryDepartmentSalaries(request, resource, user) {
  match_result = resource.match('departments/{department}/salaries')
  if match_result.matches() && request.method == 'list' {
    department = match_result.groups.department;
    return !('user' in request.params.fields)
        && hr.department(department).manager == user;
  }
  return false;
}

// ... readUserSalary ...
```

Rules may also be written as assignments or as anonymous blocks:

```
// Assignment style rule which supports named evaluation.
@rule authenticated = request.auth != null ? allow : defer;

// Block style rules are useful when all @rule statements will be evaluated
// together. The last statement in the block is treated as the rule decision.
@rule {
  // Note: the authenticated rule included by name.
  // This rule allows requests for resources marked public, otherwise denies.
  return !authenticated && resource.name.contains('/public/') 
       ? allow : deny; 
}
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

A decision is a return statement within an `@rule`. The decision types across
`@rule` declarations *should* agree. In the case of multiple `@rule`s being
evaluated where the outputs of each do not agree, the decision is considered
dynamically typed. For authorization rules, decisions are often boolean or
three-valued. In the three-value case the decision may be one of: `allow`,
`deny`, and `defer` where the last decision type defers the `allow` or `deny`
decision to subsequent `@rule`s.

The decision semantics when multiple `@rule`s are evaluated depends on the
conflict resolution algorithm used when executing rules.

Note: decision resolution across `@rule`s are as yet undefined, but will be
addressed in a future update to the proposal.

# Triggers

Triggers are functions that emit obligations and advice to be performed based
on the decision. The functions emit contextual information about the operation
to be performed and the impact on the overall request behavior. The terms
obligations and advice align with the concepts introduced within
[XACML](xacml.org).  Although XACML does not specify the order of execution or
conflict resolution behavior among obligations, this can radically affect
request behavior. Future revisions of this spec should address the different
kinds of obligations that may be emitted and conflict resolution strategies to
be employed during execution, but for now they are broken down into two
categories: synchronous and asynchronous. Synchronous obligations may include
preconditions, whereas async obligations would be considered promises. Advice,
on the other hand, should always be considered asynchronous and best-effort.

In the rule examples, the decision is binary, so the first argument to the
trigger has been labelled allow although it could be any type of decision. The
second argument is a dynamic object representing context supplied to the
rules responsible for the decision.

```
// Triggers are evaluated after rule decisions and have a void return.
@trigger function onSalaryRequest(decision, ctx) {
  // Triggers may invoke rules without affecting the rule outcome.
  if (ctx.request.method == 'list'
      && !queryDepartmentSalaries(ctx.request, ctx.resource, ctx.user)) {
    logger.log(ctx.user + " denied view salary request " + ctx.resource);
  }

  // Alternatively, triggers may simply inspect context to make a decision.
  match_result = ctx.resource.match('users/{target_user}/**');
  target_user = match_result.groups.target_user;
  if (ctx.user != target_user && decision == allow) {
    logger.log(ctx.user + " perform a salary action on " + target_user);
  }
}
```

Triggers are the only functions that can and must have a void return type. They
must not be referenced within non-trigger functions, though they may invoke other
non-rule functions including other triggers.

# Tests

Being able to verify the correctness policy-related logic is of paramount
importance. As is the ability to pose ad hoc queries with partial state. To this
end we include `@test` as supported decorator and introduce the `with-as` clause
to assist with partial state bindings required for both adhoc queries and for
function mocking.

Note: the following is under review and not yet reflected in the grammar. 

```
@rule function user_owned_action(auth, resource) {
  result = resource.matches('documents/{owner}/**')
  return (result.owner == auth.uid
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
@rule function allow_user_reads(user, request) {
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
@rule function allow_user_reads(user, request) {
  if (request.method in ['get', 'list']) {
      return (request.auth.claims.email == user
      || ctx.resource_owner == user
      || db.query("/documents/" + ctx.resource).created_by == user);
  }
  return false;
}
```

The example below shows how `acme.db` provides a library of context and
functions for use with @rule statements. The `@extern` decorator is equivalent
to a forward declaration, both to serve as documentation for what exists, but
also to be consumed during type-checking to ensure the system context and
function hooks are being used correctly within @rule statements.

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
@rule {
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
    := package? import* syntax decls
    ; 
package
    := 'package' qualified_id ';' 
    ;
import
    := 'import' qualified_id ('as' id)? ';' 
    ;
syntax
    := 'syntax' '=' STRING ';' 
    ;
decls
    := (function_decl | const_decl)*
    ;
function_decl 
    := decorator? 'function' id '(' arg_list? ')' ('{' statement+ '}')?
    ;
decorator 
    := '@' id decorator_arg_list?
    ;
decorator_arg_list
    := '(' id '=' literal (',' id '=' literal)* ')'
    ;
arg_list 
    := id (',' id)*
    ;
statement 
    := condition_expr 
    |  const_decl 
    |  return_expr 
    |  comprehension_expr
    |  block_expr
    ;
condition_expr
    := filter_expr '{' statement+ '}'
    ;
filter_expr
    := 'if' expr
    ;
const_decl 
    :=  decorator? id assign_expr? ';'
    ;
assign_expr
    := '=' (expr | comprehension_expr)
    ;
return_expr
    : 'return' (expr | comprehension_expr) ';'
    ;
comprehension_expr
    := iter_expr
    |  '{' (expr '|')? iter_expr '}'
    |  '[' (expr '|')? iter_expr ']'
    ;
iter_expr
    := 'for' id (',' id)? 'in' expr filter_expr? statement?
    |  'for' id (',' id)? 'in' expr filter_expr? ('{' statement+ '}')?
    ;
block_expr
    := decorator? '{' statement+ '}'
    ;
conditional_expr
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
    := compound_expr
    |  '!'+ compound_expr
    |  '-'+ compound_expr
    ;
compound_expr
    := primary_expr
    |  compound_expr '.' id
    |  compound_expr '.' id '(' expr_list? ')'
    |  compound_expr '[' expr ']'
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
with `@trigger` must have a void return. All other functions, whether decorated
as a `@rule` or not, must have a return value. A function body consists of
any number of assignments and a single return expression. Argument and return
types are inferred based on usage.

Functions are idempotent. Given the same input, the functions must return
the same output (or in the case of triggers emit the same output). The is true
for both local and extern functions. At present functions do not support
overloads, though this may change in future iterations of this proposal.

The general form of a function is as follows:

```
function_decl 
    := decorator? 'function' id '(' arg_list? ')' ('{' statement+ '}')?
    ;
decorator 
    := '@' id decorator_arg_list?
    ;
decorator_arg_list
    := '(' id '=' literal (',' id '=' literal)* ')'
    ;
```

The set of supported decorators is limited to `@extern`, `@rule`, `@trigger`,
and `@test`. When a function may is annotated with @extern, it must be supplied
at runtime as part of the rule evaluation context. For example:

```
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
    := filter_expr
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
function name, e.g. `module.name.func()`.

Importing a module makes all symbols within that module accessible by either
the fully qualified module name, or the simple module name (the last fragment
of a qualified package identifier).


