// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
)

// indices contains a mapping of non-ground references to values to sets of bindings.
//
//  +------+------------------------------------+
//  | ref1 | val1 | bindings-1, bindings-2, ... |
//  |      +------+-----------------------------+
//  |      | val2 | bindings-m, bindings-m, ... |
//  |      +------+-----------------------------+
//  |      | .... | ...                         |
//  +------+------+-----------------------------+
//  | ref2 | .... | ...                         |
//  +------+------+-----------------------------+
//  | ...                                       |
//  +-------------------------------------------+
//
// The "value" is the data value stored at the location referred to by the ground
// reference obtained by plugging bindings into the non-ground reference that is the
// index key.
//
type indices struct {
	table map[int]*indicesNode
}

type indicesNode struct {
	key  ast.Ref
	val  *bindingIndex
	next *indicesNode
}

// NewIndices returns an empty indices.
func newIndices() *indices {
	return &indices{
		table: map[int]*indicesNode{},
	}
}

// Build initializes the references' index by walking the store for the reference and
// creating the index that maps values to bindings.
func (ind *indices) Build(store Store, txn Transaction, ref ast.Ref) error {
	index := newBindingIndex()
	ind.registerTriggers(store)
	err := iterStorage(store, txn, ref, ast.EmptyRef(), ast.NewValueMap(), func(bindings *ast.ValueMap, val interface{}) {
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
func (ind *indices) Drop(ref ast.Ref) {
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
func (ind *indices) Get(ref ast.Ref) *bindingIndex {
	node := ind.getNode(ref)
	if node != nil {
		return node.val
	}
	return nil
}

// Iter calls the iter function for each of the indices.
func (ind *indices) Iter(iter func(ast.Ref, *bindingIndex) error) error {
	for _, head := range ind.table {
		for entry := head; entry != nil; entry = entry.next {
			if err := iter(entry.key, entry.val); err != nil {
				return err
			}
		}
	}
	return nil
}

func (ind *indices) String() string {
	buf := []string{}
	for _, head := range ind.table {
		for entry := head; entry != nil; entry = entry.next {
			str := fmt.Sprintf("%v: %v", entry.key, entry.val)
			buf = append(buf, str)
		}
	}
	return "{" + strings.Join(buf, ", ") + "}"
}

func (ind *indices) dropAll(Transaction, PatchOp, []interface{}, interface{}) error {
	ind.table = map[int]*indicesNode{}
	return nil
}

func (ind *indices) getNode(ref ast.Ref) *indicesNode {
	hashCode := ref.Hash()
	for entry := ind.table[hashCode]; entry != nil; entry = entry.next {
		if entry.key.Equal(ref) {
			return entry
		}
	}
	return nil
}

const (
	triggerID = "org.openpolicyagent/index-maintenance"
)

func (ind *indices) registerTriggers(store Store) error {
	return store.Register(triggerID, TriggerConfig{
		Before: ind.dropAll,
	})
}

// bindingIndex contains a mapping of values to bindings.
type bindingIndex struct {
	table map[int]*indexNode
}

type indexNode struct {
	key  interface{}
	val  *bindingSet
	next *indexNode
}

// newBindingIndex returns a new empty index.
func newBindingIndex() *bindingIndex {
	return &bindingIndex{
		table: map[int]*indexNode{},
	}
}

// Add updates the index to include new bindings for the value.
// If the bindings already exist for the value, no change is made.
func (ind *bindingIndex) Add(val interface{}, bindings *ast.ValueMap) {

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
func (ind *bindingIndex) Iter(val interface{}, iter func(*ast.ValueMap) error) error {
	node := ind.getNode(val)
	if node == nil {
		return nil
	}
	return node.val.Iter(iter)
}

func (ind *bindingIndex) getNode(val interface{}) *indexNode {
	hashCode := hash(val)
	head := ind.table[hashCode]
	for entry := head; entry != nil; entry = entry.next {
		if util.Compare(entry.key, val) == 0 {
			return entry
		}
	}
	return nil
}

func (ind *bindingIndex) String() string {

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
	val  *ast.ValueMap
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

func (set *bindingSet) Add(val *ast.ValueMap) {
	node := set.getNode(val)
	if node != nil {
		return
	}
	hashCode := val.Hash()
	head := set.table[hashCode]
	set.table[hashCode] = &bindingSetNode{val, head}
}

func (set *bindingSet) Iter(iter func(*ast.ValueMap) error) error {
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
	set.Iter(func(bindings *ast.ValueMap) error {
		buf = append(buf, bindings.String())
		return nil
	})
	return "{" + strings.Join(buf, ", ") + "}"
}

func (set *bindingSet) getNode(val *ast.ValueMap) *bindingSetNode {
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

func iterStorage(store Store, txn Transaction, ref ast.Ref, path ast.Ref, bindings *ast.ValueMap, iter func(*ast.ValueMap, interface{})) error {

	if len(ref) == 0 {
		node, err := store.Read(txn, path)
		if err != nil {
			if IsNotFound(err) {
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
		return iterStorage(store, txn, tail, path, bindings, iter)
	}

	node, err := store.Read(txn, path)
	if err != nil {
		if IsNotFound(err) {
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
			err := iterStorage(store, txn, tail, path, cpy, iter)
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
			err := iterStorage(store, txn, tail, path, cpy, iter)
			if err != nil {
				return err
			}
			path = path[:len(path)-1]
		}
	}

	return nil
}
