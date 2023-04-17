package lcss

import (
	"reflect"
	"testing"
)

func assertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %v got %v", expected, actual)
	}
}

func TestCharNodeAdd(t *testing.T) {
	node := &charNode{}
	node.Add([]byte("abc"))
	assertEqual(t, byte(0), node.char)
	assertEqual(t, 0, node.used)
	assertEqual(t, 1, len(node.children))
	assertEqual(t, byte('a'), node.children[0].char)
	assertEqual(t, 1, node.children[0].used)
	assertEqual(t, 1, len(node.children[0].children))
	assertEqual(t, byte('b'), node.children[0].children[0].char)
	assertEqual(t, 1, node.children[0].children[0].used)
	assertEqual(t, 1, len(node.children[0].children[0].children))
	assertEqual(t, byte('c'), node.children[0].children[0].children[0].char)
	assertEqual(t, 1, node.children[0].children[0].children[0].used)
	assertEqual(t, 0, len(node.children[0].children[0].children[0].children))
	node.Add([]byte{})
	assertEqual(t, byte(0), node.char)
	assertEqual(t, 0, node.used)
	assertEqual(t, 1, len(node.children))
	node.Add([]byte("abd"))
	assertEqual(t, byte(0), node.char)
	assertEqual(t, 0, node.used)
	assertEqual(t, 1, len(node.children))
	assertEqual(t, byte('a'), node.children[0].char)
	assertEqual(t, 2, node.children[0].used)
	assertEqual(t, 1, len(node.children[0].children))
	assertEqual(t, byte('b'), node.children[0].children[0].char)
	assertEqual(t, 2, node.children[0].children[0].used)
	assertEqual(t, 2, len(node.children[0].children[0].children))
	assertEqual(t, byte('c'), node.children[0].children[0].children[0].char)
	assertEqual(t, 1, node.children[0].children[0].children[0].used)
	assertEqual(t, 0, len(node.children[0].children[0].children[0].children))
	assertEqual(t, byte('d'), node.children[0].children[0].children[1].char)
	assertEqual(t, 1, node.children[0].children[0].children[1].used)
	assertEqual(t, 0, len(node.children[0].children[0].children[1].children))
	node.Add([]byte("abc"))
	assertEqual(t, byte(0), node.char)
	assertEqual(t, 0, node.used)
	assertEqual(t, 1, len(node.children))
	assertEqual(t, byte('a'), node.children[0].char)
	assertEqual(t, 3, node.children[0].used)
	assertEqual(t, 1, len(node.children[0].children))
	assertEqual(t, byte('b'), node.children[0].children[0].char)
	assertEqual(t, 3, node.children[0].children[0].used)
	assertEqual(t, 2, len(node.children[0].children[0].children))
	assertEqual(t, byte('c'), node.children[0].children[0].children[0].char)
	assertEqual(t, 2, node.children[0].children[0].children[0].used)
	assertEqual(t, 0, len(node.children[0].children[0].children[0].children))
	assertEqual(t, byte('d'), node.children[0].children[0].children[1].char)
	assertEqual(t, 1, node.children[0].children[0].children[1].used)
	assertEqual(t, 0, len(node.children[0].children[0].children[1].children))
}

func TestCharNodeRemove(t *testing.T) {
	node := &charNode{}
	node.Add([]byte("abc"))
	node.Remove([]byte("abc"))
	assertEqual(t, byte(0), node.char)
	assertEqual(t, 0, node.used)
	assertEqual(t, 0, len(node.children))
	node.Add([]byte("abc"))
	node.Add([]byte("abd"))
	node.Remove([]byte("abc"))
	assertEqual(t, byte(0), node.char)
	assertEqual(t, 0, node.used)
	assertEqual(t, 1, len(node.children))
	assertEqual(t, byte('a'), node.children[0].char)
	assertEqual(t, 1, node.children[0].used)
	assertEqual(t, 1, len(node.children[0].children))
	assertEqual(t, byte('b'), node.children[0].children[0].char)
	assertEqual(t, 1, node.children[0].children[0].used)
	assertEqual(t, 1, len(node.children[0].children[0].children))
	assertEqual(t, byte('d'), node.children[0].children[0].children[0].char)
	assertEqual(t, 1, node.children[0].children[0].children[0].used)
	assertEqual(t, 0, len(node.children[0].children[0].children[0].children))
	node.Remove([]byte{})
	assertEqual(t, byte(0), node.char)
	assertEqual(t, 0, node.used)
	assertEqual(t, 1, len(node.children))
	assertEqual(t, byte('a'), node.children[0].char)
	assertEqual(t, 1, node.children[0].used)
	assertEqual(t, 1, len(node.children[0].children))
	assertEqual(t, byte('b'), node.children[0].children[0].char)
	assertEqual(t, 1, node.children[0].children[0].used)
	assertEqual(t, 1, len(node.children[0].children[0].children))
	assertEqual(t, byte('d'), node.children[0].children[0].children[0].char)
	assertEqual(t, 1, node.children[0].children[0].children[0].used)
	assertEqual(t, 0, len(node.children[0].children[0].children[0].children))
	node.Add([]byte("ab"))
	node.Remove([]byte("ab"))
	assertEqual(t, byte(0), node.char)
	assertEqual(t, 0, node.used)
	assertEqual(t, 1, len(node.children))
	assertEqual(t, byte('a'), node.children[0].char)
	assertEqual(t, 1, node.children[0].used)
	assertEqual(t, 1, len(node.children[0].children))
	assertEqual(t, byte('b'), node.children[0].children[0].char)
	assertEqual(t, 1, node.children[0].children[0].used)
	assertEqual(t, 1, len(node.children[0].children[0].children))
	assertEqual(t, byte('d'), node.children[0].children[0].children[0].char)
	assertEqual(t, 1, node.children[0].children[0].children[0].used)
	assertEqual(t, 0, len(node.children[0].children[0].children[0].children))
	node.Add([]byte("ab"))
	node.Remove([]byte("abd"))
	assertEqual(t, byte(0), node.char)
	assertEqual(t, 0, node.used)
	assertEqual(t, 1, len(node.children))
	assertEqual(t, byte('a'), node.children[0].char)
	assertEqual(t, 1, node.children[0].used)
	assertEqual(t, 1, len(node.children[0].children))
	assertEqual(t, byte('b'), node.children[0].children[0].char)
	assertEqual(t, 1, node.children[0].children[0].used)
	assertEqual(t, 0, len(node.children[0].children[0].children))
}

func TestCharNodeLongestCommonPrefixLength(t *testing.T) {
	node := &charNode{}
	assertEqual(t, 0, node.LongestCommonPrefixLength())
	node.Add([]byte("abc"))
	assertEqual(t, 3, node.LongestCommonPrefixLength())
	node.Add([]byte("abd"))
	assertEqual(t, 2, node.LongestCommonPrefixLength())
	node.Remove([]byte("abd"))
	assertEqual(t, 3, node.LongestCommonPrefixLength())
	node.Add([]byte("ab"))
	assertEqual(t, 2, node.LongestCommonPrefixLength())
}

func TestCharNodeLongestCommonPrefix(t *testing.T) {
	node := &charNode{}
	assertEqual(t, []byte{}, node.LongestCommonPrefix())
	node.Add([]byte("abc"))
	assertEqual(t, []byte("abc"), node.LongestCommonPrefix())
	node.Add([]byte("abd"))
	assertEqual(t, []byte("ab"), node.LongestCommonPrefix())
	node.Remove([]byte("abd"))
	assertEqual(t, []byte("abc"), node.LongestCommonPrefix())
	node.Add([]byte("ab"))
	assertEqual(t, []byte("ab"), node.LongestCommonPrefix())
}

func TestCharNodeBug1(t *testing.T) {
	node := &charNode{}
	node.Add([]byte("a"))
	node.Add([]byte("a"))
	node.Remove([]byte("a"))
	node.Add([]byte("abbara"))
	node.Add([]byte("abr"))

	assertEqual(t, byte(0), node.char)
	assertEqual(t, 0, node.used)
	assertEqual(t, 1, len(node.children))
	assertEqual(t, byte('a'), node.children[0].char)
	assertEqual(t, 3, node.children[0].used)
	assertEqual(t, 1, len(node.children[0].children))
	assertEqual(t, byte('b'), node.children[0].children[0].char)
	assertEqual(t, 2, node.children[0].children[0].used)
	assertEqual(t, 2, len(node.children[0].children[0].children))
	assertEqual(t, byte('b'), node.children[0].children[0].children[0].char)
	assertEqual(t, 1, node.children[0].children[0].children[0].used)
	assertEqual(t, 1, len(node.children[0].children[0].children[0].children))
	assertEqual(t, byte('r'), node.children[0].children[0].children[1].char)
	assertEqual(t, 1, node.children[0].children[0].children[1].used)
	assertEqual(t, 0, len(node.children[0].children[0].children[1].children))
	assertEqual(t, 1, node.LongestCommonPrefixLength())
	assertEqual(t, []byte("a"), node.LongestCommonPrefix())
}

func TestCharNodeBug2(t *testing.T) {
	node := &charNode{}
	node.Add([]byte("habrahabr"))
	node.Add([]byte("bbara"))
	node.Add([]byte("mraja"))
	node.Remove([]byte("habrahabr"))
	node.Add([]byte("r"))
	node.Remove([]byte("bbara"))
	node.Add([]byte("ra"))
	node.Remove([]byte("r"))
	node.Add([]byte("rahabr"))
	node.Remove([]byte("bbara"))
	node.Add([]byte("ra"))
	node.Remove([]byte("mraja"))
	node.Add([]byte("raja"))

	assertEqual(t, byte(0), node.char)
	assertEqual(t, 0, node.used)
	assertEqual(t, 1, len(node.children))
	assertEqual(t, byte('r'), node.children[0].char)
	assertEqual(t, 3, node.children[0].used)
	assertEqual(t, 1, len(node.children[0].children))
	assertEqual(t, byte('a'), node.children[0].children[0].char)
	assertEqual(t, 3, node.children[0].children[0].used)
	assertEqual(t, 2, len(node.children[0].children[0].children))
	assertEqual(t, byte('h'), node.children[0].children[0].children[0].char)
	assertEqual(t, 1, node.children[0].children[0].children[0].used)
	assertEqual(t, 1, len(node.children[0].children[0].children[0].children))
	assertEqual(t, byte('j'), node.children[0].children[0].children[1].char)
	assertEqual(t, 1, node.children[0].children[0].children[1].used)
	assertEqual(t, 1, len(node.children[0].children[0].children[1].children))
	assertEqual(t, []byte("ra"), node.LongestCommonPrefix())
}

func TestLongestCommonSubstring(t *testing.T) {
	assertEqual(t, []byte{}, LongestCommonSubstring())
	assertEqual(t, []byte("abc"), LongestCommonSubstring([]byte("abc")))
	assertEqual(t, []byte{}, LongestCommonSubstring([]byte("abc"), []byte{}))
	assertEqual(t, []byte("ab"), LongestCommonSubstring([]byte("abc"), []byte("abd")))
	assertEqual(t, []byte("bc"), LongestCommonSubstring([]byte("abc"), []byte("dbc")))
	assertEqual(t, []byte("ab"), LongestCommonSubstring([]byte("ab"), []byte("abd")))
	assertEqual(t, []byte("ABC"), LongestCommonSubstring(
		[]byte("ABABC"), []byte("BABCA"), []byte("ABCBA")))
	assertEqual(t, []byte("ra"), LongestCommonSubstring(
		[]byte("habrahabr"),
		[]byte("abbara"),
		[]byte("humraja")))
	assertEqual(t, []byte("abcdez"), LongestCommonSubstring(
		[]byte("zxabcdezy"),
		[]byte("yzabcdezx"),
		[]byte("abcdez"),
		[]byte("zyzxabcdez")))
}

func TestLongestCommonSubstringWithSuffixArrays(t *testing.T) {
	assertEqual(t, []byte{}, lcss(nil, nil))
	assertEqual(t, []byte("abc"), lcss(
		[][]byte{[]byte("abc")}, [][]int{{1, 2, 3}}))
}
