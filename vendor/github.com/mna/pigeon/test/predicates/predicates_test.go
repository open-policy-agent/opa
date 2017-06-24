package predicates

import "testing"

// Go1.7: The Method and NumMethod methods of Type and Value no longer return or count unexported methods.
// So cannot use reflect.TypeOf and MethodByName to test the implemented methods.
func TestPredicatesArgs(t *testing.T) {
	var cur interface{} = &current{}
	_, ok := cur.(interface {
		onA5(interface{}) (bool, error)
		onA9(interface{}) (bool, error)
		onA13(interface{}) (bool, error)
		onB9(interface{}) (bool, error)
		onB10(interface{}) (bool, error)
		onB11(interface{}) (bool, error)
		onC1(interface{}) (interface{}, error)
	})
	if !ok {
		t.Errorf("want *current to have the expected methods")
	}
}
