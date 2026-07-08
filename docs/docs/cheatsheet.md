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
```

<RunSnippet id="preamble.rego"/>



## Rules - <sub><sup>The building blocks of Rego</sup></sub>



### Single-Value Rules


Single-value rules assign a single value. 
In older documentation, these are sometimes referred to as "complete rules". ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie1xuICBcInVzZXJcIjoge1xuICAgIFwicm9sZVwiOiBcImFkbWluXCIsXG4gICAgXCJpbnRlcm5hbFwiOiB0cnVlXG4gIH1cbn0iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5kZWZhdWx0IGFsbG93IDo9IGZhbHNlXG5cbmFsbG93IGlmIHtcblx0aW5wdXQudXNlci5yb2xlID09IFwiYWRtaW5cIlxuXHRpbnB1dC51c2VyLmludGVybmFsXG59XG5cbmRlZmF1bHQgcmVxdWVzdF9xdW90YSA6PSAxMDBcblxucmVxdWVzdF9xdW90YSA6PSAxMDAwIGlmIGlucHV0LnVzZXIuaW50ZXJuYWxcbnJlcXVlc3RfcXVvdGEgOj0gNTAgaWYgaW5wdXQudXNlci5wbGFuLnRyaWFsXG4ifQ%3D%3D))



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


Name local query variables. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5cbmFsbF9yZWdpb25zIDo9IHtcblx0XCJlbWVhXCI6IHtcIndlc3RcIiwgXCJlYXN0XCJ9LFxuXHRcIm5hXCI6IHtcIndlc3RcIiwgXCJlYXN0XCIsIFwiY2VudHJhbFwifSxcblx0XCJsYXRhbVwiOiB7XCJ3ZXN0XCIsIFwiZWFzdFwifSxcblx0XCJhcGFjXCI6IHtcIm5vcnRoXCIsIFwic291dGhcIn0sXG59XG5cbmFsbG93ZWRfcmVnaW9ucyBjb250YWlucyByZWdpb25faWQgaWYge1xuXHRzb21lIGFyZWEsIHJlZ2lvbnMgaW4gYWxsX3JlZ2lvbnNcblxuXHRzb21lIHJlZ2lvbiBpbiByZWdpb25zXG5cdHJlZ2lvbl9pZCA6PSBzcHJpbnRmKFwiJXNfJXNcIiwgW2FyZWEsIHJlZ2lvbl0pXG59XG4ifQ%3D%3D))




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


Check conditions on many elements. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie1xuICBcInVzZXJJRFwiOiBcInUxMjNcIixcbiAgXCJwYXRoc1wiOiBbXG4gICAgXCIvZG9jcy91MTIzL25vdGVzLnR4dFwiLFxuICAgIFwiL2RvY3MvdTEyMy9xNC1yZXBvcnQuZG9jeFwiXG4gIF1cbn0iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5cbmFsbG93IGlmIHtcblx0cHJlZml4IDo9IHNwcmludGYoXCIvZG9jcy8lcy9cIiwgW2lucHV0LnVzZXJJRF0pXG5cdGV2ZXJ5IHBhdGggaW4gaW5wdXQucGF0aHMge1xuXHRcdHN0YXJ0c3dpdGgocGF0aCwgcHJlZml4KVxuXHR9XG59XG4ifQ%3D%3D))



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


Statements in rules are joined with logical AND. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie1xuICBcImVtYWlsXCI6IFwiam9lQGV4YW1wbGUuY29tXCJcbn0iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5cbnZhbGlkX3N0YWZmX2VtYWlsIGlmIHtcblx0cmVnZXgubWF0Y2goYF5cXFMrQFxcUytcXC5cXFMrJGAsIGlucHV0LmVtYWlsKSAjIGFuZFxuXHRlbmRzd2l0aChpbnB1dC5lbWFpbCwgXCJleGFtcGxlLmNvbVwiKVxufVxuIn0%3D))



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


Express OR with multiple rules, functions or the in keyword. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie1xuICBcImVtYWlsXCI6IFwib3BhQGV4YW1wbGUuY29tXCIsXG4gIFwibmFtZVwiOiBcImFubmFcIixcbiAgXCJtZXRob2RcIjogXCJHRVRcIlxufSIsInAiOiJwYWNrYWdlIGNoZWF0XG5cblxuIyB1c2luZyBtdWx0aXBsZSBydWxlc1xudmFsaWRfZW1haWwgaWYgZW5kc3dpdGgoaW5wdXQuZW1haWwsIFwiQGV4YW1wbGUuY29tXCIpXG52YWxpZF9lbWFpbCBpZiBlbmRzd2l0aChpbnB1dC5lbWFpbCwgXCJAZXhhbXBsZS5vcmdcIilcbnZhbGlkX2VtYWlsIGlmIGVuZHN3aXRoKGlucHV0LmVtYWlsLCBcIkBleGFtcGxlLm5ldFwiKVxuXG4jIHVzaW5nIGZ1bmN0aW9uc1xuYWxsb3dlZF9maXJzdG5hbWUobmFtZSkgaWYge1xuXHRzdGFydHN3aXRoKG5hbWUsIFwiYVwiKVxuXHRjb3VudChuYW1lKSBcdTAwM2UgMlxufVxuXG5hbGxvd2VkX2ZpcnN0bmFtZShcImpvZVwiKSAjIGlmIG5hbWUgPT0gJ2pvZSdcblxudmFsaWRfbmFtZSBpZiBhbGxvd2VkX2ZpcnN0bmFtZShpbnB1dC5uYW1lKVxuXG52YWxpZF9yZXF1ZXN0IGlmIHtcblx0aW5wdXQubWV0aG9kIGluIHtcIkdFVFwiLCBcIlBPU1RcIn0gIyB1c2luZyBgaW5gXG59XG4ifQ%3D%3D))



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


Override input and data using the with keyword. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5cbmFsbG93IGlmIGlucHV0LmFkbWluID09IHRydWVcblxudGVzdF9hbGxvd193aGVuX2FkbWluIGlmIHtcblx0YWxsb3cgd2l0aCBpbnB1dCBhcyB7XCJhZG1pblwiOiB0cnVlfVxufVxuIn0%3D))




```rego title="policy.rego"
allow if input.admin == true

test_allow_when_admin if {
	allow with input as {"admin": true}
}
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>




## Debugging - <sub><sup>Find and fix problems</sup></sub>



### Print


Use print in rules to inspect values at runtime. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5cbmFsbG93ZWRfdXNlcnMgOj0ge1wiYWxpY2VcIiwgXCJib2JcIn1cblxuYWxsb3cgaWYge1xuXHRzb21lIHVzZXIgaW4gYWxsb3dlZF91c2Vyc1xuXHRwcmludCh1c2VyKVxuXHRpbnB1dC51c2VyID09IHVzZXJcbn1cbiJ9))




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
 ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5cbmRvdWJsZWQgOj0gW20gfFxuXHRzb21lIG4gaW4gWzEsIDIsIDMsIDNdXG5cdG0gOj0gbiAqIDJcbl1cbiJ9))




```rego title="policy.rego"
doubled := [m |
	some n in [1, 2, 3, 3]
	m := n * 2
]
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>



### Sets


Produce unordered collections without duplicates.
 ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5cbnVuaXF1ZV9kb3VibGVkIGNvbnRhaW5zIG0gaWYge1xuXHRzb21lIG4gaW4gWzEwLCAyMCwgMzAsIDIwLCAxMF1cblx0bSA6PSBuICogMlxufVxuIn0%3D))




```rego title="policy.rego"
unique_doubled contains m if {
	some n in [10, 20, 30, 20, 10]
	m := n * 2
}
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>



### Objects


Produce key:value data. Note, keys must be unique.
 ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5cbmlzX2V2ZW5bbnVtYmVyXSA6PSBpc19ldmVuIGlmIHtcblx0c29tZSBudW1iZXIgaW4gWzEsIDIsIDMsIDRdXG5cdGlzX2V2ZW4gOj0gKG51bWJlciAlIDIpID09IDBcbn1cbiJ9))




```rego title="policy.rego"
is_even[number] := is_even if {
	some number in [1, 2, 3, 4]
	is_even := (number % 2) == 0
}
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>




## Builtins - <sub><sup>Handy functions for common tasks</sup></sub>



### Regex


Pattern match and replace string data. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5cbmV4YW1wbGVfc3RyaW5nIDo9IFwiQnVpbGQgUG9saWN5IGFzIENvZGUgd2l0aCBPUEEhXCJcblxuY2hlY2tfbWF0Y2ggaWYgcmVnZXgubWF0Y2goYFxcdytgLCBleGFtcGxlX3N0cmluZylcblxuY2hlY2tfcmVwbGFjZSA6PSByZWdleC5yZXBsYWNlKGV4YW1wbGVfc3RyaW5nLCBgXFxzK2AsIFwiX1wiKVxuIn0%3D))




```rego title="policy.rego"
example_string := "Build Policy as Code with OPA!"

check_match if regex.match(`\w+`, example_string)

check_replace := regex.replace(example_string, `\s+`, "_")
```


<RunSnippet command="data.cheat" depends="preamble.rego"/>



### Strings


Check and transform strings. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5cbmV4YW1wbGVfc3RyaW5nIDo9IFwiQnVpbGQgUG9saWN5IGFzIENvZGUgd2l0aCBPUEEhXCJcblxuY2hlY2tfY29udGFpbnMgaWYgY29udGFpbnMoZXhhbXBsZV9zdHJpbmcsIFwiT1BBXCIpXG5jaGVja19zdGFydHN3aXRoIGlmIHN0YXJ0c3dpdGgoZXhhbXBsZV9zdHJpbmcsIFwiQnVpbGRcIilcbmNoZWNrX2VuZHN3aXRoIGlmIGVuZHN3aXRoKGV4YW1wbGVfc3RyaW5nLCBcIiFcIilcbmNoZWNrX3JlcGxhY2UgOj0gcmVwbGFjZShleGFtcGxlX3N0cmluZywgXCJPUEFcIiwgXCJPUEEhXCIpXG5jaGVja19zcHJpbnRmIDo9IHNwcmludGYoXCJPUEEgaXMgJXMhXCIsIFtcImF3ZXNvbWVcIl0pXG4ifQ%3D%3D))




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


Summarize data. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5cbnZhbHMgOj0gWzUsIDEsIDQsIDIsIDNdXG52YWxzX2NvdW50IDo9IGNvdW50KHZhbHMpXG52YWxzX21heCA6PSBtYXgodmFscylcbnZhbHNfbWluIDo9IG1pbih2YWxzKVxudmFsc19zb3J0ZWQgOj0gc29ydCh2YWxzKVxudmFsc19zdW0gOj0gc3VtKHZhbHMpXG4ifQ%3D%3D))




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


Work with key value and nested data. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5cbm9iaiA6PSB7XCJ1c2VyaWRcIjogXCIxODQ3MlwiLCBcInJvbGVzXCI6IFt7XCJuYW1lXCI6IFwiYWRtaW5cIn1dfVxuXG4jIHBhdGhzIGNhbiBjb250YWluIGFycmF5IGluZGV4ZXMgdG9vXG52YWwgOj0gb2JqZWN0LmdldChvYmosIFtcInJvbGVzXCIsIDAsIFwibmFtZVwiXSwgXCJtaXNzaW5nXCIpXG5cbmRlZmF1bHRlZF92YWwgOj0gb2JqZWN0LmdldChcblx0b2JqLFxuXHRbXCJyb2xlc1wiLCAwLCBcInBlcm1pc3Npb25zXCJdLCAjIHBhdGhcblx0XCJ1bmtub3duXCIsICMgZGVmYXVsdCBpZiBwYXRoIGlzIG1pc3Npbmdcbilcblxua2V5cyA6PSBvYmplY3Qua2V5cyhvYmopXG4ifQ%3D%3D))




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


Manipulate and make checks on objects. ([Try It](https://play.openpolicyagent.org/?state=eyJpIjoie30iLCJwIjoicGFja2FnZSBjaGVhdFxuXG5cbnVuaW9uZWQgOj0gb2JqZWN0LnVuaW9uKHtcImZvb1wiOiB0cnVlfSwge1wiYmFyXCI6IGZhbHNlfSlcblxuc3Vic2V0IDo9IG9iamVjdC5zdWJzZXQoXG5cdHtcImZvb1wiOiB0cnVlLCBcImJhclwiOiBmYWxzZX0sXG5cdHtcImZvb1wiOiB0cnVlfSwgIyBzdWJzZXQgb2JqZWN0XG4pXG5cbnJlbW92ZWQgOj0gb2JqZWN0LnJlbW92ZShcblx0e1wiZm9vXCI6IHRydWUsIFwiYmFyXCI6IGZhbHNlfSxcblx0e1wiYmFyXCJ9LCAjIHJlbW92ZSBrZXlzXG4pXG4ifQ%3D%3D))




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





