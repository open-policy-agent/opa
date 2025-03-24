package p

x := 1
y := 2

x1 := 1

x2 := 2

a.b.c := 1
a.b.d := 2

s contains 1
s contains 2

s contains 3

s contains 4

rule if foo == bar
rule if bar == foo

long if {
    x := 1
    y := 2
}
long if {
    x := 2
    y := 3
}

short if condition
not_short if {
	rule
	body
}

not_short if {
	rule
	body
}
short if condition

s contains "foo"
s contains "bar" if {
	foo
	bar 
}

