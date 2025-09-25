---
sidebar_label: "Cheat Sheet"
---

# Rego Cheat Sheet

<!-- The source of truth for this file's contents is https://github.com/open-policy-agent/rego-cheat-sheet -->

:::tip
**Did you know?** There's a [printable PDF](/cheatsheet.pdf) version of the
cheatsheet too!
:::

All code examples on this page share this preamble:

```rego
package cheat
import rego.v1
```

<RunSnippet id="preamble.rego"/>



## Rules - <sub><sup>The building blocks of Rego</sup></sub>



### Single-Value Rules


Single-value rules assign a single value. 
In older documentation, these are sometimes referred to as "complete rules". ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie1xuICBcInVzZXJcIjoge1xuICAgIFwicm9sZVwiOiBcImFkbWluXCIsXG4gICAgXCJpbnRlcm5hbFwiOiB0cnVlXG4gIH1cbn0iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5kZWZhdWx0IGFsbG93IDo9IGZhbHNlXG5cbmFsbG93IGlmIHtcblx0aW5wdXQudXNlci5yb2xlID09IFwiYWRtaW5cIlxuXHRpbnB1dC51c2VyLmludGVybmFsXG59XG5cbmRlZmF1bHQgcmVxdWVzdF9xdW90YSA6PSAxMDBcbnJlcXVlc3RfcXVvdGEgOj0gMTAwMCBpZiBpbnB1dC51c2VyLmludGVybmFsXG5yZXF1ZXN0X3F1b3RhIDo9IDUwIGlmIGlucHV0LnVzZXIucGxhbi50cmlhbFxuIn0%3D))




```json title="input.json"
{
  "user": {
    "role": "admin",
    "internal": true
  }
}
```

<RunSnippet id="input.Single-Value+Rules.json"/>


```rego title="policy.rego"
default allow := false

allow if {
	input.user.role == "admin"
	input.user.internal
}

default request_quota := 100
request_quota := 1000 if input.user.internal
request_quota := 50 if input.user.plan.trial
```


<RunSnippet command="data.cheat" files="#input.Single-Value+Rules.json" depends="preamble.rego"/>



### Multi-Value Set Rules


Multi-value set rules generate and assign a set of values to a variable.
In older documentation these are sometimes referred to as "partial set rules". ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie1xuICBcInVzZXJcIjoge1xuICAgIFwidGVhbXNcIjogW1xuICAgICAgXCJvcHNcIixcbiAgICAgIFwiZW5nXCJcbiAgICBdXG4gIH1cbn1cbiIsInAiOiJwYWNrYWdlIGNoZWF0XG5cbnBhdGhzIGNvbnRhaW5zIFwiL2hhbmRib29rLypcIlxuXG5wYXRocyBjb250YWlucyBwYXRoIGlmIHtcblx0c29tZSB0ZWFtIGluIGlucHV0LnVzZXIudGVhbXNcblx0cGF0aCA6PSBzcHJpbnRmKFwiL3RlYW1zLyV2LypcIiwgW3RlYW1dKVxufVxuIn0%3D))




```json title="input.json"
{
  "user": {
    "teams": [
      "ops",
      "eng"
    ]
  }
}

```

<RunSnippet id="input.Multi-Value+Set+Rules.json"/>


```rego title="policy.rego"
paths contains "/handbook/*"

paths contains path if {
	some team in input.user.teams
	path := sprintf("/teams/%v/*", [team])
}
```


<RunSnippet command="data.cheat" files="#input.Multi-Value+Set+Rules.json" depends="preamble.rego"/>



### Multi-Value Object Rules


Multi-value object rules generate and assign a set of keys and values to a variable.
In older documentation these are sometimes referred to as "partial object rules".
 ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie1xuICBcInBhdGhzXCI6IFtcbiAgICBcImEvMTIzLnR4dFwiLFxuICAgIFwiYS80NTYudHh0XCIsXG4gICAgXCJiL2Zvby50eHRcIixcbiAgICBcImIvYmFyLnR4dFwiLFxuICAgIFwiYy94LnR4dFwiXG4gIF1cbn1cbiIsInAiOiJwYWNrYWdlIGNoZWF0XG5cbiMgQ3JlYXRlcyBhbiBvYmplY3Qgd2l0aCBzZXRzIGFzIHRoZSB2YWx1ZXMuXG5wYXRoc19ieV9wcmVmaXhbcHJlZml4XSBjb250YWlucyBwYXRoIGlmIHtcblx0c29tZSBwYXRoIGluIGlucHV0LnBhdGhzXG5cdHBhcnRzIDo9IHNwbGl0KHBhdGgsIFwiL1wiKVxuXHRwcmVmaXggOj0gcGFydHNbMF1cbn1cbiJ9))




```json title="input.json"
{
  "paths": [
    "a/123.txt",
    "a/456.txt",
    "b/foo.txt",
    "b/bar.txt",
    "c/x.txt"
  ]
}

```

<RunSnippet id="input.Multi-Value+Object+Rules.json"/>


```rego title="policy.rego"
# Creates an object with sets as the values.
paths_by_prefix[prefix] contains path if {
	some path in input.paths
	parts := split(path, "/")
	prefix := parts[0]
}
```


<RunSnippet command="data.cheat" files="#input.Multi-Value+Object+Rules.json" depends="preamble.rego"/>




## Iteration - <sub><sup>Make quick work of collections</sup></sub>



### Some


Name local query variables. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5pbXBvcnQgcmVnby52MVxuXG5hbGxfcmVnaW9ucyA6PSB7XG5cdFwiZW1lYVwiOiB7XCJ3ZXN0XCIsIFwiZWFzdFwifSxcblx0XCJuYVwiOiB7XCJ3ZXN0XCIsIFwiZWFzdFwiLCBcImNlbnRyYWxcIn0sXG5cdFwibGF0YW1cIjoge1wid2VzdFwiLCBcImVhc3RcIn0sXG5cdFwiYXBhY1wiOiB7XCJub3J0aFwiLCBcInNvdXRoXCJ9LFxufVxuXG5hbGxvd2VkX3JlZ2lvbnMgY29udGFpbnMgcmVnaW9uX2lkIGlmIHtcblx0c29tZSBhcmVhLCByZWdpb25zIGluIGFsbF9yZWdpb25zXG5cblx0c29tZSByZWdpb24gaW4gcmVnaW9uc1xuXHRyZWdpb25faWQgOj0gc3ByaW50ZihcIiVzXyVzXCIsIFthcmVhLCByZWdpb25dKVxufVxuIn0%3D))




```rego title="policy.rego"
all_regions := {
	"emea": {"west", "east"},
	"na": {"west", "east", "central"},
	"latam": {"west", "east"},
	"apac": {"north", "south"},
}

allowed_regions contains region_id if {
	some area, regions in all_regions

	some region in regions
	region_id := sprintf("%s_%s", [area, region])
}
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>



### Every


Check conditions on many elements. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie1xuICBcInVzZXJJRFwiOiBcInUxMjNcIixcbiAgXCJwYXRoc1wiOiBbXG4gICAgXCIvZG9jcy91MTIzL25vdGVzLnR4dFwiLFxuICAgIFwiL2RvY3MvdTEyMy9xNC1yZXBvcnQuZG9jeFwiXG4gIF1cbn0iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5pbXBvcnQgcmVnby52MVxuXG5hbGxvdyBpZiB7XG5cdHByZWZpeCA6PSBzcHJpbnRmKFwiL2RvY3MvJXMvXCIsIFtpbnB1dC51c2VySURdKVxuXHRldmVyeSBwYXRoIGluIGlucHV0LnBhdGhzIHtcblx0XHRzdGFydHN3aXRoKHBhdGgsIHByZWZpeClcblx0fVxufVxuIn0%3D))




```json title="input.json"
{
  "userID": "u123",
  "paths": [
    "/docs/u123/notes.txt",
    "/docs/u123/q4-report.docx"
  ]
}
```

<RunSnippet id="input.Every.json"/>


```rego title="policy.rego"
allow if {
	prefix := sprintf("/docs/%s/", [input.userID])
	every path in input.paths {
		startswith(path, prefix)
	}
}
```


<RunSnippet command="data.cheat" files="#input.Every.json" depends="preamble.rego"/>




## Control Flow - <sub><sup>Handle different conditions</sup></sub>



### Logical AND


Statements in rules are joined with logical AND. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie1xuICBcImVtYWlsXCI6IFwiam9lQGV4YW1wbGUuY29tXCJcbn0iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5pbXBvcnQgcmVnby52MVxuXG52YWxpZF9zdGFmZl9lbWFpbCBpZiB7XG5cdHJlZ2V4Lm1hdGNoKGBeXFxTK0BcXFMrXFwuXFxTKyRgLCBpbnB1dC5lbWFpbCkgIyBhbmRcblx0ZW5kc3dpdGgoaW5wdXQuZW1haWwsIFwiZXhhbXBsZS5jb21cIilcbn1cbiJ9))




```json title="input.json"
{
  "email": "joe@example.com"
}
```

<RunSnippet id="input.Logical+AND.json"/>


```rego title="policy.rego"
valid_staff_email if {
	regex.match(`^\S+@\S+\.\S+$`, input.email) # and
	endswith(input.email, "example.com")
}
```


<RunSnippet command="data.cheat" files="#input.Logical+AND.json" depends="preamble.rego"/>



### Logical OR


Express OR with multiple rules, functions or the in keyword. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie1xuICBcImVtYWlsXCI6IFwib3BhQGV4YW1wbGUuY29tXCIsXG4gIFwibmFtZVwiOiBcImFubmFcIixcbiAgXCJtZXRob2RcIjogXCJHRVRcIlxufSIsInAiOiJwYWNrYWdlIGNoZWF0XG5cbmltcG9ydCByZWdvLnYxXG5cbiMgdXNpbmcgbXVsdGlwbGUgcnVsZXNcbnZhbGlkX2VtYWlsIGlmIGVuZHN3aXRoKGlucHV0LmVtYWlsLCBcIkBleGFtcGxlLmNvbVwiKVxudmFsaWRfZW1haWwgaWYgZW5kc3dpdGgoaW5wdXQuZW1haWwsIFwiQGV4YW1wbGUub3JnXCIpXG52YWxpZF9lbWFpbCBpZiBlbmRzd2l0aChpbnB1dC5lbWFpbCwgXCJAZXhhbXBsZS5uZXRcIilcblxuIyB1c2luZyBmdW5jdGlvbnNcbmFsbG93ZWRfZmlyc3RuYW1lKG5hbWUpIGlmIHtcblx0c3RhcnRzd2l0aChuYW1lLCBcImFcIilcblx0Y291bnQobmFtZSkgXHUwMDNlIDJcbn1cblxuYWxsb3dlZF9maXJzdG5hbWUoXCJqb2VcIikgIyBpZiBuYW1lID09ICdqb2UnXG5cbnZhbGlkX25hbWUgaWYgYWxsb3dlZF9maXJzdG5hbWUoaW5wdXQubmFtZSlcblxudmFsaWRfcmVxdWVzdCBpZiB7XG5cdGlucHV0Lm1ldGhvZCBpbiB7XCJHRVRcIiwgXCJQT1NUXCJ9ICMgdXNpbmcgYGluYFxufVxuIn0%3D))




```json title="input.json"
{
  "email": "opa@example.com",
  "name": "anna",
  "method": "GET"
}
```

<RunSnippet id="input.Logical+OR.json"/>


```rego title="policy.rego"
# using multiple rules
valid_email if endswith(input.email, "@example.com")
valid_email if endswith(input.email, "@example.org")
valid_email if endswith(input.email, "@example.net")

# using functions
allowed_firstname(name) if {
	startswith(name, "a")
	count(name) > 2
}

allowed_firstname("joe") # if name == 'joe'

valid_name if allowed_firstname(input.name)

valid_request if {
	input.method in {"GET", "POST"} # using `in`
}
```


<RunSnippet command="data.cheat" files="#input.Logical+OR.json" depends="preamble.rego"/>




## Testing - <sub><sup>Validate your policy's behavior</sup></sub>



### With


Override input and data using the with keyword. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5pbXBvcnQgcmVnby52MVxuXG5hbGxvdyBpZiBpbnB1dC5hZG1pbiA9PSB0cnVlXG5cbnRlc3RfYWxsb3dfd2hlbl9hZG1pbiBpZiB7XG5cdGFsbG93IHdpdGggaW5wdXQgYXMge1wiYWRtaW5cIjogdHJ1ZX1cbn1cbiJ9))




```rego title="policy.rego"
allow if input.admin == true

test_allow_when_admin if {
	allow with input as {"admin": true}
}
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>




## Debugging - <sub><sup>Find and fix problems</sup></sub>



### Print


Use print in rules to inspect values at runtime. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5pbXBvcnQgcmVnby52MVxuXG5hbGxvd2VkX3VzZXJzIDo9IHtcImFsaWNlXCIsIFwiYm9iXCJ9XG5cbmFsbG93IGlmIHtcblx0c29tZSB1c2VyIGluIGFsbG93ZWRfdXNlcnNcblx0cHJpbnQodXNlcilcblx0aW5wdXQudXNlciA9PSB1c2VyXG59XG4ifQ%3D%3D))




```rego title="policy.rego"
allowed_users := {"alice", "bob"}

allow if {
	some user in allowed_users
	print(user)
	input.user == user
}
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>




## Comprehensions - <sub><sup>Rework and process collections</sup></sub>



### Arrays


Produce ordered collections, maintaining duplicates.
 ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5pbXBvcnQgcmVnby52MVxuXG5kb3VibGVkIDo9IFttIHxcblx0c29tZSBuIGluIFsxLCAyLCAzLCAzXVxuXHRtIDo9IG4gKiAyXG5dXG4ifQ%3D%3D))




```rego title="policy.rego"
doubled := [m |
	some n in [1, 2, 3, 3]
	m := n * 2
]
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>



### Sets


Produce unordered collections without duplicates.
 ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5pbXBvcnQgcmVnby52MVxuXG51bmlxdWVfZG91YmxlZCBjb250YWlucyBtIGlmIHtcblx0c29tZSBuIGluIFsxMCwgMjAsIDMwLCAyMCwgMTBdXG5cdG0gOj0gbiAqIDJcbn1cbiJ9))




```rego title="policy.rego"
unique_doubled contains m if {
	some n in [10, 20, 30, 20, 10]
	m := n * 2
}
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>



### Objects


Produce key:value data. Note, keys must be unique.
 ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5pbXBvcnQgcmVnby52MVxuXG5pc19ldmVuW251bWJlcl0gOj0gaXNfZXZlbiBpZiB7XG5cdHNvbWUgbnVtYmVyIGluIFsxLCAyLCAzLCA0XVxuXHRpc19ldmVuIDo9IChudW1iZXIgJSAyKSA9PSAwXG59XG4ifQ%3D%3D))




```rego title="policy.rego"
is_even[number] := is_even if {
	some number in [1, 2, 3, 4]
	is_even := (number % 2) == 0
}
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>




## Builtins - <sub><sup>Handy functions for common tasks</sup></sub>



### Regex


Pattern match and replace string data. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5pbXBvcnQgcmVnby52MVxuXG5leGFtcGxlX3N0cmluZyA6PSBcIkJ1aWxkIFBvbGljeSBhcyBDb2RlIHdpdGggT1BBIVwiXG5cbmNoZWNrX21hdGNoIGlmIHJlZ2V4Lm1hdGNoKGBcXHcrYCwgZXhhbXBsZV9zdHJpbmcpXG5cbmNoZWNrX3JlcGxhY2UgOj0gcmVnZXgucmVwbGFjZShleGFtcGxlX3N0cmluZywgYFxccytgLCBcIl9cIilcbiJ9))




```rego title="policy.rego"
example_string := "Build Policy as Code with OPA!"

check_match if regex.match(`\w+`, example_string)

check_replace := regex.replace(example_string, `\s+`, "_")
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>



### Strings


Check and transform strings. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5pbXBvcnQgcmVnby52MVxuXG5leGFtcGxlX3N0cmluZyA6PSBcIkJ1aWxkIFBvbGljeSBhcyBDb2RlIHdpdGggT1BBIVwiXG5cbmNoZWNrX2NvbnRhaW5zIGlmIGNvbnRhaW5zKGV4YW1wbGVfc3RyaW5nLCBcIk9QQVwiKVxuY2hlY2tfc3RhcnRzd2l0aCBpZiBzdGFydHN3aXRoKGV4YW1wbGVfc3RyaW5nLCBcIkJ1aWxkXCIpXG5jaGVja19lbmRzd2l0aCBpZiBlbmRzd2l0aChleGFtcGxlX3N0cmluZywgXCIhXCIpXG5jaGVja19yZXBsYWNlIDo9IHJlcGxhY2UoZXhhbXBsZV9zdHJpbmcsIFwiT1BBXCIsIFwiT1BBIVwiKVxuY2hlY2tfc3ByaW50ZiA6PSBzcHJpbnRmKFwiT1BBIGlzICVzIVwiLCBbXCJhd2Vzb21lXCJdKVxuIn0%3D))




```rego title="policy.rego"
example_string := "Build Policy as Code with OPA!"

check_contains if contains(example_string, "OPA")
check_startswith if startswith(example_string, "Build")
check_endswith if endswith(example_string, "!")
check_replace := replace(example_string, "OPA", "OPA!")
check_sprintf := sprintf("OPA is %s!", ["awesome"])
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>



### Aggregates


Summarize data. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5pbXBvcnQgcmVnby52MVxuXG52YWxzIDo9IFs1LCAxLCA0LCAyLCAzXVxudmFsc19jb3VudCA6PSBjb3VudCh2YWxzKVxudmFsc19tYXggOj0gbWF4KHZhbHMpXG52YWxzX21pbiA6PSBtaW4odmFscylcbnZhbHNfc29ydGVkIDo9IHNvcnQodmFscylcbnZhbHNfc3VtIDo9IHN1bSh2YWxzKVxuIn0%3D))




```rego title="policy.rego"
vals := [5, 1, 4, 2, 3]
vals_count := count(vals)
vals_max := max(vals)
vals_min := min(vals)
vals_sorted := sort(vals)
vals_sum := sum(vals)
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>



### Objects: Extracting Data


Work with key value and nested data. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5pbXBvcnQgcmVnby52MVxuXG5vYmogOj0ge1widXNlcmlkXCI6IFwiMTg0NzJcIiwgXCJyb2xlc1wiOiBbe1wibmFtZVwiOiBcImFkbWluXCJ9XX1cblxuIyBwYXRocyBjYW4gY29udGFpbiBhcnJheSBpbmRleGVzIHRvb1xudmFsIDo9IG9iamVjdC5nZXQob2JqLCBbXCJyb2xlc1wiLCAwLCBcIm5hbWVcIl0sIFwibWlzc2luZ1wiKVxuXG5kZWZhdWx0ZWRfdmFsIDo9IG9iamVjdC5nZXQoXG5cdG9iaixcblx0W1wicm9sZXNcIiwgMCwgXCJwZXJtaXNzaW9uc1wiXSwgIyBwYXRoXG5cdFwidW5rbm93blwiLCAjIGRlZmF1bHQgaWYgcGF0aCBpcyBtaXNzaW5nXG4pXG5cbmtleXMgOj0gb2JqZWN0LmtleXMob2JqKVxuIn0%3D))




```rego title="policy.rego"
obj := {"userid": "18472", "roles": [{"name": "admin"}]}

# paths can contain array indexes too
val := object.get(obj, ["roles", 0, "name"], "missing")

defaulted_val := object.get(
	obj,
	["roles", 0, "permissions"], # path
	"unknown", # default if path is missing
)

keys := object.keys(obj)
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>



### Objects: Transforming Data


Manipulate and make checks on objects. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5pbXBvcnQgcmVnby52MVxuXG51bmlvbmVkIDo9IG9iamVjdC51bmlvbih7XCJmb29cIjogdHJ1ZX0sIHtcImJhclwiOiBmYWxzZX0pXG5cbnN1YnNldCA6PSBvYmplY3Quc3Vic2V0KFxuXHR7XCJmb29cIjogdHJ1ZSwgXCJiYXJcIjogZmFsc2V9LFxuXHR7XCJmb29cIjogdHJ1ZX0sICMgc3Vic2V0IG9iamVjdFxuKVxuXG5yZW1vdmVkIDo9IG9iamVjdC5yZW1vdmUoXG5cdHtcImZvb1wiOiB0cnVlLCBcImJhclwiOiBmYWxzZX0sXG5cdHtcImJhclwifSwgIyByZW1vdmUga2V5c1xuKVxuIn0%3D))




```rego title="policy.rego"
unioned := object.union({"foo": true}, {"bar": false})

subset := object.subset(
	{"foo": true, "bar": false},
	{"foo": true}, # subset object
)

removed := object.remove(
	{"foo": true, "bar": false},
	{"bar"}, # remove keys
)
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>





