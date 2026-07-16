---
title: "Introducing the OPA print function"
authors: ["anderseknert"]
date: 2021-10-29
slug: introducing-the-opa-print-function-809da6a13aee
---

![Banner image for introducing the OPA print function post](/img/blog/introducing-the-opa-print-function-809da6a13aee/banner.webp)

One of the key takeaways from the [Open Policy Agent 2021 Survey](/blog/open-policy-agent-2021-survey-summary-e749bbd7b824), was the need to improve the OPA debugging experience. Simply put, we need to make it easier to know what's going on when policies and rules are evaluated.

However, whenever someone talks about an "experience," it's rarely a small task and a checkbox to be checked once completed. Rather, it's all the little things that when combined provide a great improvement to the greater goal. If the OPA project used JIRA, it would probably be a safe bet to classify the "improve debugging experience" story as an "epic." With some improvements made, many new ideas and feature requests are likely to emerge along the way, and it would be rather optimistic to think that such a story ever got _done_, in the sense that no new improvements could be made.

To make things more complicated — and certainly more interesting — the OPA debugging experience isn't isolated to OPA itself. Improving the debugging experience for OPA entails not just looking at where things can be made better in OPA, but just as much in the tools commonly used when authoring Rego policies. These include tools like [VS Code](https://marketplace.visualstudio.com/items?itemName=tsandall.opa), [IntelliJ IDEA](https://plugins.jetbrains.com/plugin/14865-open-policy-agent) and all the [other editors](https://www.openpolicyagent.org/docs/latest/editor-and-ide-support/) commonly used for policy authoring.

So, where do we start?

## Debugging with OPA eval

Evaluating rules and variables has traditionally been done using the aptly named **opa eval** command. Commonly referred to as the "Swiss army knife of OPA," opa eval allows a policy author to quickly evaluate either standalone expressions like:

```bash
opa eval --format raw 1+1
```

Or, more commonly, with policy, data and input provided through command line arguments, and the path to the rule or variable of interest.

```bash
opa eval --data policy.rego --input input.json data.policy.main
```

While truly a versatile tool, debugging with **opa eval** has a couple of drawbacks. Having to create a new file to provide input might feel a little clunky, but hardly a terrible experience. But what if you want to evaluate the value of some variable inside of a rule, a test or a comprehension?

The [trace](https://www.openpolicyagent.org/docs/latest/policy-reference/#debugging) built-in has to some extent been used for this purpose, but requires additional parameters passed to OPA to actually print something, and while occasionally useful, it was often perceived as somewhat clunky for the purpose of simply printing something.

Another problem frequently mentioned in the context of debugging is how sometimes **opa eval**, or the **trace** built-in comes back with just… nothing.

This brings us right into another topic often considered tricky with regards to debugging — OPA's handling of undefined. When OPA encounters undefined values, policy evaluation normally halts. Considering how Rego is a declarative _query_ language, there isn't a whole lot more to do when a query comes back with nothing, just like a query language like SQL wouldn't have a whole lot more to do with an empty resultset. Since Rego rules are often compositions of many statements or other rules, it can sometimes be pretty difficult to tell where the undefined value that halted policy evaluation was introduced.

What to do?

## Introducing the print built-in

To tackle this, OPA v0.34.0 introduces a new **print** function to its ever growing list of [built-ins](https://www.openpolicyagent.org/docs/latest/policy-reference/#built-in-functions). The print function does exactly what you'd expect it to do — prints any provided values to the console. Consider a rule like the one below.

```rego
allow {
    print("Entering allow")
    role := input.user.roles[_]
    print("Found role", role)
    role == "admin"
}
```

Running OPA eval would produce the following result.

```bash
$ opa eval -f raw -d policy.rego -i input.json 'data.policy.allow'
Entering allow
Found role developer
Found role sysadmin
Found role dba
Found role admin
true
```

The **print** function takes any number of arguments (static values, variables, **input**, **data**, etc) and prints each one (separated by whitespace) to the console.

While simple on the surface, a whole lot of [thought](https://github.com/open-policy-agent/opa/issues/3319) has been put into its design, and unlike other built-in functions (which are often trivial to add into OPA) the print function required changes to the internal compiler. How come?

## Varargs

One of the design goals of the new print function was to allow a variable number of values or variables (i.e. varargs) to be passed as arguments, without resorting to the use of an array for the arguments, as is done by **sprintf** and other built-ins. Simply put we wanted something intuitive like:

```rego
print("x", input.x, "y", input.y)
```

To work just as expected. Sounds easy, right? Well, not really.

One of the more obscure (and hence, not encouraged) features of Rego can be traced back to its Datalog roots. Any built-in function can have it's return value expressed as the last argument to the function. Meaning that:

```rego
x := concat(".", ["a", "b", "c"])
```

Could alternatively be written as:

```rego
concat(".", ["a", "b", "c"], x)
```

With the last argument "reserved" for the return value, how would an implementation of varargs work? The answer was a new type of void function, where there simply is no return value to take into account. Since Rego functions are generally free from side effects, a void type of function hasn't really made sense previously, but with **print** having no purpose other than the desired side effect of printing to the console, adding a void type made sense.

## Printing undefined

The next problem to tackle in order for print to work nicely as a debugging tool was how to deal with undefined. Since we can expect print to be used to debug variables from **input** and **data** that might not be defined, it would be kind of a bummer if calling print _itself_ halted policy evaluation! Ideally we'd be able to call the print function and have it print _something_ even if some of the arguments provided pointed at undefined values. That way we could use the function to try and help also with the problem of identifying where in a policy undefined values have been introduced.

This requirement meant some internal assumptions of how Rego is parsed had to change, and the end result is a print function that prints undefined values as `<undefined>`, without halting policy evaluation.

```rego
allow {
    print(input.user.email, input.user.roles)
    input.user.roles[_] == "admin"
    endswith(input.user.email, "@acmecorp.com")
}
```

Evaluating the above allow rule with user.email missing from the input would now output something like this to the console:

```bash
$ opa eval -f raw -d policy.rego -i input.json data.policy.allow
<undefined> ["developer", "admin"]
```

## Using print

OPA supports many different modes of operation, from **opa eval** and **opa test**, to the OPA REPL and of course running as a standalone server. Both **opa eval** and the REPL will always print to the console (stderr, specifically) as expected. When running as a server, OPA will print any output from print function calls at the **info** log level. This makes print useful for debugging at the default info level or below. When configured to run with log level **error** (the generally recommended log level for production), OPA erases any calls to print from policies as they are loaded. Print calls left in the policy at that point will thus not impact performance whatsoever.

When running **opa test**, the **print** function by default will print to the console on test failures. Should you want to print output also for successful tests, the — verbose (short form -v) will do the trick. One case I've found particularly useful in tests is to use print in combination with the with … as mocking construct, to quickly see what exactly the result of a rule evaluation returns, like:

```rego
test_decison_allowed {
    result := decision with input as {
        "user": {
            "id": "abc123"
        },
        "request": {
            "method": "POST",
            "path": "/users"
        }
    }
    print(result)
    print(expectedResult)

    result == expectedResult
}

decision {
    [_, payload, _] := io.jwt.decode(input.user.token)
    print(payload)
    ...
}
```

While printing the outcome of rule evaluation in tests like this is valuable, the decision rule _itself_ might be composed of multiple rules, and being able to add a few print lines in those to understand why our result isn't what we expect can help us quickly pinpoint the problem.

## Wrapping up

Rego as a policy language isn't a general purpose programming language, and shouldn't be treated as one. However, making the OPA and Rego debugging experience as smooth as possible means we sometimes might need to adapt concepts familiar from the programming languages policy authors normally work with. That the **print** function [pull request](https://github.com/open-policy-agent/opa/pull/3868) added almost 2000 lines of code — half of them however from added test cases! — to the codebase is an interesting case study in how all the "little" details — like backwards compatibility, usability and performance — need to be considered when adding new functionality to a mature open source project like OPA.

I hope that you'll find the print function a useful addition to OPA, and a small improvement to the OPA/Rego debugging experience. Expect a lot more to come out in this space in future releases, and as always, make your voice heard in the [OPA Slack](https://slack.openpolicyagent.org/) if you have ideas, questions or feature requests you'd like to see incorporated into OPA!
