// Test of redundant if err != nil

// Package pkg ...
package pkg

func f() error {
	if err := f(); err != nil {
		g()
		return err
	}
	return nil
}

func g() error {
	if err := f(); err != nil { // MATCH /redundant/
		return err
	}
	return nil
}

func h() error {
	if err, x := f(), 1; err != nil {
		return err
	}
	return nil
}

func i() error {
	a := 1
	if err := f(); err != nil {
		a++
		return err
	}
	return nil
}

func j() error {
	var a error
	if err := f(); err != nil {
		return err
	}
	return a
}

func k() error {
	if err := f(); err != nil {
		// TODO: handle error better
		return err
	}
	return nil
}

func l() (interface{}, error) {
	if err := f(); err != nil {
		return nil, err
	}
	if err := f(); err != nil {
		return nil, err
	}
	if err := f(); err != nil {
		return nil, err
	}
	// Phew, it worked
	return nil
}

func m() error {
	if err := f(); err != nil {
		return err
	}
	if err := f(); err != nil {
		return err
	}
	if err := f(); err != nil {
		return err
	}
	// Phew, it worked again.
	return nil
}

func multi() error {
	a := 0
	var err error
	// unreachable code after return statements is intentional to check that it
	// doesn't confuse the linter.
	if true {
		a++
		if err := f(); err != nil { // MATCH /redundant/
			return err
		}
		return nil
		a++
	} else {
		a++
		if err = f(); err != nil { // MATCH /redundant/
			return err
		}
		return nil
		a++
	}
}
