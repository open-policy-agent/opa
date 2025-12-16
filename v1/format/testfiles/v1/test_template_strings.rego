package test

a := $""

a2 := $``

b := $"{"foo"} {`bar`} {42} {4.2} {true} {null}"

b2 := $`{"foo"} {`bar`} {42} {4.2} {true} {null}`

c := $"foo\n{x}"

c2 := $`foo
{x}`

d := $"{[1,   2,3,	4]} {{1,   2,3,	4}} {{"a":   `foo`,	`b`:"bar"}}"

e := $"{[1,   2,
3,	4]} {{1,   2,
3,	4}} {{"a":   `foo`,
`b`:"bar"}}"

f := $"{[x |
x := ["a", "b"][_]]}"

g := $"{# this is a comment
       x} {# so is this
y # this one too
# this one third
} {
# this one fourth
z
}"

h := $"{# this is a comment
       "foo"} {# so is this
42
} {
# this one too
true
}"

i := $"{

x

}"
