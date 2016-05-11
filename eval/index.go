// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package eval

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/open-policy-agent/opa/ast"
)

// Indices contains a mapping of non-ground references to values to sets of bindings.
//
// +------+------------------------------------+
// | ref1 | val1 | bindings-1, bindings-2, ... |
// |      +------+-----------------------------+
// |      | val2 | bindings-m, bindings-m, ... |
// |      +------+-----------------------------+
// |      | .... | ...                         |
// +------+------+-----------------------------+
// | ref2 | .... | ...                         |
// +------+------+-----------------------------+
// | ...     								   |
// +-------------------------------------------+
//
// The "value" is the data value stored at the location referred to by the ground
// reference obtained by plugging bindings into the non-ground reference that is the
// index key.
//
type Indices struct {
	table map[int]*indicesNode
}

type indicesNode struct {
	key  ast.Ref
	val  *Index
	next *indicesNode
}

// NewIndices returns an empty indices.
func NewIndices() *Indices {
	return &Indices{
		table: map[int]*indicesNode{},
	}
}

// Build initializes the references' index by walking the store for the reference and
// creating the index that maps values to bindings.
func (ind *Indices) Build(store *Storage, ref ast.Ref) error {
	index := NewIndex()
	err := iterStorage(store, ref, ast.EmptyRef(), NewBindings(), func(bindings *Bindings, val interface{}) {
		index.Add(val, bindings)
	})
	if err != nil {
		return err
	}
	hashCode := ref.Hash()
	head := ind.table[hashCode]
	entry := &indicesNode{
		key:  ref,
		val:  index,
		next: head,
	}
	ind.table[hashCode] = entry
	return nil
}

// Drop removes the index for the reference.
func (ind *Indices) Drop(ref ast.Ref) {
	hashCode := ref.Hash()
	var prev *indicesNode
	for entry := ind.table[hashCode]; entry != nil; entry = entry.next {
		if entry.key.Equal(ref) {
			if prev == nil {
				ind.table[hashCode] = entry.next
			} else {
				prev.next = entry.next
			}
			return
		}
	}
}

// Get returns the reference's index.
func (ind *Indices) Get(ref ast.Ref) *Index {
	node := ind.getNode(ref)
	if node != nil {
		return node.val
	}
	return nil
}

// Iter calls the iter function for each of the indices.
func (ind *Indices) Iter(iter func(ast.Ref, *Index) error) error {
	for _, head := range ind.table {
		for entry := head; entry != nil; entry = entry.next {
			if err := iter(entry.key, entry.val); err != nil {
				return err
			}
		}
	}
	return nil
}

func (ind *Indices) String() string {
	buf := []string{}
	for _, head := range ind.table {
		for entry := head; entry != nil; entry = entry.next {
			str := fmt.Sprintf("%v: %v", entry.key, entry.val)
			buf = append(buf, str)
		}
	}
	return "{" + strings.Join(buf, ", ") + "}"
}

func (ind *Indices) getNode(ref ast.Ref) *indicesNode {
	hashCode := ref.Hash()
	for entry := ind.table[hashCode]; entry != nil; entry = entry.next {
		if entry.key.Equal(ref) {
			return entry
		}
	}
	return nil
}

// Index contains a mapping of values to bindings.
//
type Index struct {
	table map[int]*indexNode
}

type indexNode struct {
	key  interface{}
	val  *bindingSet
	next *indexNode
}

// NewIndex returns a new empty index.
func NewIndex() *Index {
	return &Index{
		table: map[int]*indexNode{},
	}
}

// Add updates the index to include new bindings for the value.
// If the bindings already exist for the value, no change is made.
func (ind *Index) Add(val interface{}, bindings *Bindings) {

	node := ind.getNode(val)
	if node != nil {
		node.val.Add(bindings)
		return
	}

	hashCode := hash(val)
	bindingsSet := newBindingSet()
	bindingsSet.Add(bindings)

	entry := &indexNode{
		key:  val,
		val:  bindingsSet,
		next: ind.table[hashCode],
	}

	ind.table[hashCode] = entry
}

// Iter calls the iter function for each set of bindings for the value.
func (ind *Index) Iter(val interface{}, iter func(*Bindings) error) error {
	node := ind.getNode(val)
	if node == nil {
		return nil
	}
	return node.val.Iter(iter)
}

func (ind *Index) getNode(val interface{}) *indexNode {
	hashCode := hash(val)
	head := ind.table[hashCode]
	for entry := head; entry != nil; entry = entry.next {
		if Compare(entry.key, val) == 0 {
			return entry
		}
	}
	return nil
}

func (ind *Index) String() string {

	buf := []string{}

	for _, head := range ind.table {
		for entry := head; entry != nil; entry = entry.next {
			str := fmt.Sprintf("%v: %v", entry.key, entry.val)
			buf = append(buf, str)
		}
	}

	return "{" + strings.Join(buf, ", ") + "}"
}

type bindingSetNode struct {
	val  *Bindings
	next *bindingSetNode
}

type bindingSet struct {
	table map[int]*bindingSetNode
}

func newBindingSet() *bindingSet {
	return &bindingSet{
		table: map[int]*bindingSetNode{},
	}
}

func (set *bindingSet) Add(val *Bindings) {
	node := set.getNode(val)
	if node != nil {
		return
	}
	hashCode := val.Hash()
	head := set.table[hashCode]
	set.table[hashCode] = &bindingSetNode{val, head}
}

func (set *bindingSet) Iter(iter func(*Bindings) error) error {
	for _, head := range set.table {
		for entry := head; entry != nil; entry = entry.next {
			if err := iter(entry.val); err != nil {
				return err
			}
		}
	}
	return nil
}

func (set *bindingSet) String() string {
	buf := []string{}
	set.Iter(func(bindings *Bindings) error {
		buf = append(buf, bindings.String())
		return nil
	})
	return "{" + strings.Join(buf, ", ") + "}"
}

func (set *bindingSet) getNode(val *Bindings) *bindingSetNode {
	hashCode := val.Hash()
	for entry := set.table[hashCode]; entry != nil; entry = entry.next {
		if entry.val.Equal(val) {
			return entry
		}
	}
	return nil
}

func hash(v interface{}) int {
	switch v := v.(type) {
	case []interface{}:
		var h int
		for _, e := range v {
			h += hash(e)
		}
		return h
	case map[string]interface{}:
		var h int
		for k, v := range v {
			h += hash(k) + hash(v)
		}
		return h
	case string:
		h := fnv.New64a()
		h.Write([]byte(v))
		return int(h.Sum64())
	case bool:
		if v {
			return 1
		}
		return 0
	case nil:
		return 0
	case float64:
		return int(v)
	}
	panic(fmt.Sprintf("illegal argument: %v (%T)", v, v))
}

func iterStorage(store *Storage, ref ast.Ref, path ast.Ref, bindings *Bindings, iter func(*Bindings, interface{})) error {

	if len(ref) == 0 {
		node, err := lookup(store, path)
		if err != nil {
			if IsStorageNotFound(err) {
				return nil
			}
			return err
		}

		iter(bindings, node)
		return nil
	}

	head := ref[0]
	tail := ref[1:]

	headVar, isVar := head.Value.(ast.Var)

	if !isVar || len(path) == 0 {
		path = append(path, head)
		return iterStorage(store, tail, path, bindings, iter)
	}

	node, err := lookup(store, path)
	if err != nil {
		if IsStorageNotFound(err) {
			return nil
		}
		return err
	}

	switch node := node.(type) {
	case map[string]interface{}:
		for key := range node {
			path = append(path, ast.StringTerm(key))
			cpy := bindings.Copy()
			cpy.Put(headVar, ast.String(key))
			err := iterStorage(store, tail, path, cpy, iter)
			if err != nil {
				return err
			}
			path = path[:len(path)-1]
		}
	case []interface{}:
		for i := range node {
			path = append(path, ast.NumberTerm(float64(i)))
			cpy := bindings.Copy()
			cpy.Put(headVar, ast.Number(float64(i)))
			err := iterStorage(store, tail, path, cpy, iter)
			if err != nil {
				return err
			}
			path = path[:len(path)-1]
		}
	}

	return nil
}
