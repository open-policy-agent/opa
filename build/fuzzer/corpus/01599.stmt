package ex

		foo(x) = y {
			split(x, "i", y)
		}

		bar[x] = y {
			data.l[_].a = x
			foo(x, y)
		}

		chain0(x) = y {
			foo(x, y)
		}

		chain1(a) = b {
			chain0(a, b)
		}

		chain2 = d {
			chain1("fooibar", d)
		}

		cross(x) = [a, b] {
			split(x, "i", y)
			foo(y[1], b)
			data.test.foo(y[2], a)
		}

		falsy_func(x) = false

		falsy_func_else(x) = true { x = 1 } else = false { true }

		falsy_undefined {
			falsy_func(1)
		}

		falsy_negation {
			not falsy_func(1)
		}

		falsy_else_value = falsy_func_else(2)

		falsy_else_undefined {
			falsy_func_else(2)
		}

		falsy_else_negation {
			not falsy_func_else(2)
		}

		arrays([x, y]) = [a, b] {
			foo(x, a)
			foo(y, b)
		}

		arraysrule = y {
			arrays(["hih", "foo"], y)
		}

		objects({"foo": x, "bar": y}) = z {
			foo(x, a)
			data.test.foo(y, b)
			z = [a, b]
		}

		objectsrule = y {
			objects({"foo": "hih", "bar": "hi ho"}, y)
		}

		refoutput = y {
			foo("hih", z)
			y = z[1]
		}

		void(x) {
			x = "foo"
		}

		voidGood {
			not void("bar", true)
		}

		voidBad {
			void("bar", true)
		}

		multi(1, x) = y {
			y = x
		}

		multi(2, x) = y {
			a = 2*x
			y = a+1
		}

		multi(3, x) = y {
			y = x*10
		}

		multi("foo", x) = y {
			y = "bar"
		}

		multi1 = y {
			multi(1, 2, y)
		}

		multi2 = y {
			multi(2, 2, y)
		}

		multi3 = y {
			multi(3, 2, y)
		}

		multi4 = y {
			multi("foo", 2, y)
		}

		always_true_fn(x)

		always_true {
			always_true_fn(1)
		}
		