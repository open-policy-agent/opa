package p

f1(x) = x

f2(x) := x

f3(1)

f4(1) {
	true
}

f5(1) := x {
	x := 5
}

f6(x) {
	true
} else := false

f7(x) := 1 {
	x.key1
} else := false {
	x.key2
} else {
	false
}
