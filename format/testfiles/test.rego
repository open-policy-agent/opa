# The blank lines below me should be gone! (except one)


# Comment!
package a.b

# I also, am a comment.
import data.x.y.z
import data.a.b.c              # Another comment!
# I belong with data.a, there should be a newline before me.
import data.a
import data.f.g

default foo = false
foo[x] {
not x = g
    f(x) = 1
    g(
        x, "foo"
    ) = z
}

globals = {"foo": "bar",
"fizz": "buzz"}

partial_obj["x"] = 1
partial_obj.y = 2

partial_obj["z"] = 3 {
    true
}

partial_set["x"]
partial_set.y

# Latent comment.

r = y {
    y = x
    split("foo.bar", ".", input.x) with input as {"x": x}
    }

# Comment on else
        else = y {
    y = ["howdy"]
    x = {"x": {
    "y": "z",
    }}
    a = {"a": {
    "b": "c",
    }, "b": "c", "c": [1, 2,
    3, 4]}
    }

fn(x) = y {
		y = x
}

long(x) = true {
    x = "foo %host"
    }

    short(x) {
    x = "bar"
    }

raw_string = `hi\there`
raw_multiline = `this
string
  is on
multiple lines`

fn2([x, y,
z], {"foo": a}) = b {
split(x, y, c)
trim(a, z, d) # function comment 1
split(c[0], d, b)
x = sprintf("hello %v",
["world"])
#function comment 2
} # function comment 3

f[x] {
    x = "hi"
} { # Comment on chain
    x = "bye"
}

        import data.foo.bar
    import data.bar.foo # data.bar.foo should be first

p[x] = y { y = x
                y = "foo"
                z = { "a": "b", # Comment inside object 1
                    "b":    "c"   ,   "c": "d",   # comment on object entry line
                    # Comment inside object 2
"d": "e",
} # Comment on closing object brace.
a = {"a": "b", "c": "d"}
b = [1, 2, 3, 4]
c = [1, 2,
# Comment inside array
3, 4,
5, 6, 7,
8,
] # Comment on closing array bracket.

d = [1 | b[_]]
e = [1 | split("foo.bar", ".", x); x[_]]
f = [1 | split("foo.bar", ".", x)
x[_]]
g = [1 |
split("foo.bar", ".", x) # comment in array comprehension
x[_]
# inner comment
]

h = {1 | b[_]}
i = {1 | split("foo.bar", ".", x); x[_]}
j = {1 | split("foo.bar", ".", x)
x[_]}
k = {1 |
split("foo.bar", ".", x) # comment in set comprehension
x[_]

# inner comment
}

l = {"foo":1 | b[_]}
m = {y:
x | split("foo.bar", ".", x); y = x[_]}
n = {y:  x | split("foo.bar", ".", x)
y = x[_]}
o = {y: x |
split("foo.bar", ".", x) # comment in object comprehension
y = x[_]

# inner comment
}
} # Comment on rule closing brace

# more comments!
# more comments!
# more comments!
# more comments!
