package buffer

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestBuffer_FromUint(t *testing.T) {
	b := FromUint(1)
	if bytes.Compare([]byte{1}, b.Bytes()) != 0 {
		t.Fatal("mismatched buffer values")
	}
}

func TestBuffer_Convert(t *testing.T) {
	v1 := []byte{'a', 'b', 'c'}
	b := Buffer(v1)
	if bytes.Compare(v1, b.Bytes()) != 0 {
		t.Fatal("mismatched buffer values")
	}

	v2 := "abc"
	b = Buffer(v2)
	if bytes.Compare([]byte(v2), b.Bytes()) != 0 {
		t.Fatal("mismatched buffer values")
	}
}

func TestBuffer_Base64Encode(t *testing.T) {
	b := Buffer{'a', 'b', 'c'}
	v, err := b.Base64Encode()
	if err != nil {
		t.Fatal("failed to base64 encode")
	}
	if bytes.Compare([]byte{'Y', 'W', 'J', 'j'}, v) != 0 {
		t.Fatal("mismatched buffer values")
	}
}

func TestJSON(t *testing.T) {
	b1 := Buffer{'a', 'b', 'c'}

	jsontxt, err := json.Marshal(b1)
	if err != nil {
		t.Fatal("failed to marshal buffer")
	}
	if `"YWJj"` != string(jsontxt) {
		t.Fatal("mismatched json values")
	}

	var b2 Buffer
	err = json.Unmarshal(jsontxt, &b2)
	if err != nil {
		t.Fatal("failed to marshal buffer")
	}

	if bytes.Compare(b1, b2) != 0 {
		t.Fatal("mismatched buffer values")
	}
}

func TestFunky(t *testing.T) {
	s := `QD4_B3ghg0PNu-c_EAlXn3Xlb0gzAFPJSYQSI1cZZ8sPIxISgPMtNJTzgncC281IaKDXLV1aEnYuH5eH-4u4f383zlyBCGKSKSQWmqKNE7xcIqleFVNsfzOucTL4QRxfbcyHcli_symC_RGWJ6GdocE0VgyYN8t9_0sm_Nq5lcwtYEQs_hNlf1ileCjjdsUfC05zTbbrLpMjgI3IK5_QxOU81FLei4LMx3iQ1kqrIGH5FxxQMKGdx_fDaRQ-YBAA2YVqn7rs3TcwQ7NUjjz8JyDE168NlMV1WxoDC9nwOe0O6K4NzFuWpoGHTh0M-0lT5M3dy9kEBYgPtWoe_u9dogA`
	b := Buffer{}
	err := b.Base64Decode([]byte(s))
	if err != nil {
		t.Fatal("failed to base64 decode")
	}
	if 257 != b.Len() {
		t.Fatal("Mismatched buffer lengths")
	}
}

func TestBuffer_NData(t *testing.T) {
	payload := []byte("Alice")
	nd := Buffer(payload).NData()
	if bytes.Compare([]byte{0, 0, 0, 5, 65, 108, 105, 99, 101}, nd) != 0 {
		t.Fatal("mismatched byte buffer values")
	}

	b1, err := FromNData(nd)
	if err != nil {
		t.Fatal("failed to extract data")
	}
	if bytes.Compare(payload, b1.Bytes()) != 0 {
		t.Fatal("mismatched byte values ")
	}
}
