package maxexprcnt

import "testing"

func TestMaxExprCnt(t *testing.T) {
	_, err := Parse("", []byte("infinite parse"), MaxExpressions(5))
	if err == nil {
		t.Fatalf("expected non nil error message for testing max expr cnt option.")
	}

	errs, ok := err.(errList)
	if !ok {
		t.Fatalf("expected err %v to be of type errList but got type %T", err, err)
	}

	var found bool
	for _, err := range errs {
		pe, ok := err.(*parserError)
		if !ok {
			t.Fatalf("expected err %v to be of type parserError but got type %T", err, err)
		}

		if pe.Inner == errMaxExprCnt {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected to find errMaxExprCnt %v in error list %v", errMaxExprCnt, errs)
	}
}
