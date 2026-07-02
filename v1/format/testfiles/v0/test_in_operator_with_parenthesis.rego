package test

import future.keywords.in

z {
    { 1, 2 in [2, 2, 2] }
    { 1, (1, 2 in [2, 2, 2]) }
    { (x, x in [2]) | x := numbers.range(1,10)[_]}
    { x: (x, x in [2]) | x := numbers.range(1,10)[_]}
    { (x in [2]): 2 | x := numbers.range(1,10)[_]}
    { (x, x in [2]): 2 | x := numbers.range(1,10)[_]}
    { (1, 2 in [2, 2, 2]) }
    { 1, (2 in [2, 2, 2]) }
    f(1, 2 in [2, 2])
    g((1, 2 in [2, 2]))
}

f(_, _) = true
g(_) = true