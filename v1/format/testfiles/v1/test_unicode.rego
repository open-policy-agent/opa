package test

x := "\u0000"
x := "\u0000 \""

authorize = "\u0000" if {
    input.user == "\u0000"
} else = "\u0000" if {
    input.path[0] == "\u0000"
    input.source_network == "\u0000"
}

_fg := {
    "black":    "\u001b[30m",
    "red":      "\u001b[31m",
    "green":    "\u001b[32m",
    "yellow":   "\u001b[33m",
    "blue":     "\u001b[34m",
    "magenta":  "\u001b[35m",
    "cyan":     "\u001b[36m",
    "white":    "\u001b[37m",
}
