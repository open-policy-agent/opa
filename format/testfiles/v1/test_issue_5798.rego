package test

rule01 = fail
if {
    fail = {                # this
        x |                 # panics
            set[x]; f(x)
    }
}
