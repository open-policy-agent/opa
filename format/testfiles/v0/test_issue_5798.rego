package test

rule01 = fail
{
    fail = {                # this
        x |                 # panics
            set[x]; f(x)
    }
}
