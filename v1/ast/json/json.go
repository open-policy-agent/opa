// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// This package provides options for JSON marshalling of AST nodes, and location
// data in particular. Since location data occupies a significant portion of the
// AST when included, it is excluded by default. The options provided here allow
// changing that behavior — either for all nodes or for specific types. Since
// JSONMarshaller implementations have access only to the node being marshaled,
// our options are to either attach these settings to *all* nodes in the AST, or
// to provide them via global state. The former is perhaps a little more elegant,
// and is what we went with initially. The cost of attaching these settings to
// every node however turned out to be non-negligible, and given that the number
// of users who have an interest in AST serialization are likely to be few, we
// have since switched to using global state, as provided here. Note that this
// is mostly to provide an equivalent feature to what we had before, should
// anyone depend on that. Users who need fine-grained control over AST
// serialization are recommended to use external libraries for that purpose,
// such as `github.com/json-iterator/go`.
package json

import "sync"

// Options defines the options for JSON operations,
// currently only marshaling can be configured
type Options struct {
	MarshalOptions MarshalOptions
}

// MarshalOptions defines the options for JSON marshaling,
// currently only toggling the marshaling of location information is supported
type MarshalOptions struct {
	// IncludeLocation toggles the marshaling of location information
	IncludeLocation NodeToggle
	// IncludeLocationText additionally/optionally includes the text of the location
	IncludeLocationText bool
	// ExcludeLocationFile additionally/optionally excludes the file of the location
	// Note that this is inverted (i.e. not "include" as the default needs to remain false)
	ExcludeLocationFile bool
}

// NodeToggle is a generic struct to allow the toggling of
// settings for different ast node types
// Optimized to use bitflags instead of individual bools to save memory
type NodeToggle struct {
	flags uint16
}

// Bit positions for each node type
const (
	toggleTerm uint16 = 1 << iota
	togglePackage
	toggleComment
	toggleImport
	toggleRule
	toggleHead
	toggleExpr
	toggleSomeDecl
	toggleEvery
	toggleWith
	toggleAnnotations
	toggleAnnotationsRef
)

// Getters for each field
func (n NodeToggle) Term() bool           { return n.flags&toggleTerm != 0 }
func (n NodeToggle) Package() bool        { return n.flags&togglePackage != 0 }
func (n NodeToggle) Comment() bool        { return n.flags&toggleComment != 0 }
func (n NodeToggle) Import() bool         { return n.flags&toggleImport != 0 }
func (n NodeToggle) Rule() bool           { return n.flags&toggleRule != 0 }
func (n NodeToggle) Head() bool           { return n.flags&toggleHead != 0 }
func (n NodeToggle) Expr() bool           { return n.flags&toggleExpr != 0 }
func (n NodeToggle) SomeDecl() bool       { return n.flags&toggleSomeDecl != 0 }
func (n NodeToggle) Every() bool          { return n.flags&toggleEvery != 0 }
func (n NodeToggle) With() bool           { return n.flags&toggleWith != 0 }
func (n NodeToggle) Annotations() bool    { return n.flags&toggleAnnotations != 0 }
func (n NodeToggle) AnnotationsRef() bool { return n.flags&toggleAnnotationsRef != 0 }

// Setters for each field
func (n *NodeToggle) SetTerm(v bool)           { n.setFlag(toggleTerm, v) }
func (n *NodeToggle) SetPackage(v bool)        { n.setFlag(togglePackage, v) }
func (n *NodeToggle) SetComment(v bool)        { n.setFlag(toggleComment, v) }
func (n *NodeToggle) SetImport(v bool)         { n.setFlag(toggleImport, v) }
func (n *NodeToggle) SetRule(v bool)           { n.setFlag(toggleRule, v) }
func (n *NodeToggle) SetHead(v bool)           { n.setFlag(toggleHead, v) }
func (n *NodeToggle) SetExpr(v bool)           { n.setFlag(toggleExpr, v) }
func (n *NodeToggle) SetSomeDecl(v bool)       { n.setFlag(toggleSomeDecl, v) }
func (n *NodeToggle) SetEvery(v bool)          { n.setFlag(toggleEvery, v) }
func (n *NodeToggle) SetWith(v bool)           { n.setFlag(toggleWith, v) }
func (n *NodeToggle) SetAnnotations(v bool)    { n.setFlag(toggleAnnotations, v) }
func (n *NodeToggle) SetAnnotationsRef(v bool) { n.setFlag(toggleAnnotationsRef, v) }

func (n *NodeToggle) setFlag(flag uint16, value bool) {
	if value {
		n.flags |= flag
	} else {
		n.flags &^= flag
	}
}

// NewNodeToggle creates a NodeToggle with specific fields enabled
// This is a convenience function for tests and external users
func NewNodeToggle() NodeToggle {
	return NodeToggle{}
}

// WithTerm enables Term flag
func (n NodeToggle) WithTerm() NodeToggle {
	n.SetTerm(true)
	return n
}

// WithPackage enables Package flag
func (n NodeToggle) WithPackage() NodeToggle {
	n.SetPackage(true)
	return n
}

// WithComment enables Comment flag
func (n NodeToggle) WithComment() NodeToggle {
	n.SetComment(true)
	return n
}

// WithImport enables Import flag
func (n NodeToggle) WithImport() NodeToggle {
	n.SetImport(true)
	return n
}

// WithRule enables Rule flag
func (n NodeToggle) WithRule() NodeToggle {
	n.SetRule(true)
	return n
}

// WithHead enables Head flag
func (n NodeToggle) WithHead() NodeToggle {
	n.SetHead(true)
	return n
}

// WithExpr enables Expr flag
func (n NodeToggle) WithExpr() NodeToggle {
	n.SetExpr(true)
	return n
}

// WithSomeDecl enables SomeDecl flag
func (n NodeToggle) WithSomeDecl() NodeToggle {
	n.SetSomeDecl(true)
	return n
}

// WithEvery enables Every flag
func (n NodeToggle) WithEvery() NodeToggle {
	n.SetEvery(true)
	return n
}

// WithWith enables With flag
func (n NodeToggle) WithWith() NodeToggle {
	n.SetWith(true)
	return n
}

// WithAnnotations enables Annotations flag
func (n NodeToggle) WithAnnotations() NodeToggle {
	n.SetAnnotations(true)
	return n
}

// WithAnnotationsRef enables AnnotationsRef flag
func (n NodeToggle) WithAnnotationsRef() NodeToggle {
	n.SetAnnotationsRef(true)
	return n
}

// WithAll enables all flags
func (n NodeToggle) WithAll() NodeToggle {
	n.flags = 0xFFFF
	return n
}

// Helper functions for creating NodeToggle with specific fields
// These are mainly for backward compatibility with tests and external code

// NodeToggleFor creates a NodeToggle with specific fields set to true
// This is a convenience function that accepts field names as parameters
func NodeToggleFor(fields ...string) NodeToggle {
	var nt NodeToggle
	for _, field := range fields {
		switch field {
		case "Term":
			nt.SetTerm(true)
		case "Package":
			nt.SetPackage(true)
		case "Comment":
			nt.SetComment(true)
		case "Import":
			nt.SetImport(true)
		case "Rule":
			nt.SetRule(true)
		case "Head":
			nt.SetHead(true)
		case "Expr":
			nt.SetExpr(true)
		case "SomeDecl":
			nt.SetSomeDecl(true)
		case "Every":
			nt.SetEvery(true)
		case "With":
			nt.SetWith(true)
		case "Annotations":
			nt.SetAnnotations(true)
		case "AnnotationsRef":
			nt.SetAnnotationsRef(true)
		}
	}
	return nt
}

// NodeToggleFromFields creates NodeToggle from field values
// Deprecated: Use builder pattern with NewNodeToggle().WithX() instead
func NodeToggleFromFields(
	term, pkg, comment, imp, rule, head, expr, someDecl, every, with, annotations, annotationsRef bool,
) NodeToggle {
	var nt NodeToggle
	if term {
		nt.SetTerm(true)
	}
	if pkg {
		nt.SetPackage(true)
	}
	if comment {
		nt.SetComment(true)
	}
	if imp {
		nt.SetImport(true)
	}
	if rule {
		nt.SetRule(true)
	}
	if head {
		nt.SetHead(true)
	}
	if expr {
		nt.SetExpr(true)
	}
	if someDecl {
		nt.SetSomeDecl(true)
	}
	if every {
		nt.SetEvery(true)
	}
	if with {
		nt.SetWith(true)
	}
	if annotations {
		nt.SetAnnotations(true)
	}
	if annotationsRef {
		nt.SetAnnotationsRef(true)
	}
	return nt
}

// configuredJSONOptions synchronizes access to the global JSON options
type configuredJSONOptions struct {
	options Options
	lock    sync.RWMutex
}

var options = &configuredJSONOptions{
	options: Defaults(),
}

// SetOptions sets the global options for marshalling AST nodes to JSON
func SetOptions(opts Options) {
	options.lock.Lock()
	defer options.lock.Unlock()
	options.options = opts
}

// GetOptions returns (a copy of) the global options for marshalling AST nodes to JSON
func GetOptions() Options {
	options.lock.RLock()
	defer options.lock.RUnlock()
	return options.options
}

// Defaults returns the default JSON options, which is to exclude location
// information in serialized JSON AST.
func Defaults() Options {
	// NodeToggle is zero-initialized with all flags set to false,
	// which is the default behavior we want
	return Options{
		MarshalOptions: MarshalOptions{
			IncludeLocation:     NodeToggle{},
			IncludeLocationText: false,
			ExcludeLocationFile: false,
		},
	}
}

// Simple constructor functions for common NodeToggle patterns used in tests
func TermLocation() NodeToggle {
	var nt NodeToggle
	nt.SetTerm(true)
	return nt
}

func PackageLocation() NodeToggle {
	var nt NodeToggle
	nt.SetPackage(true)
	return nt
}

func CommentLocation() NodeToggle {
	var nt NodeToggle
	nt.SetComment(true)
	return nt
}

func ImportLocation() NodeToggle {
	var nt NodeToggle
	nt.SetImport(true)
	return nt
}

func RuleLocation() NodeToggle {
	var nt NodeToggle
	nt.SetRule(true)
	return nt
}

func HeadLocation() NodeToggle {
	var nt NodeToggle
	nt.SetHead(true)
	return nt
}

func ExprLocation() NodeToggle {
	var nt NodeToggle
	nt.SetExpr(true)
	return nt
}

func SomeDeclLocation(enabled bool) NodeToggle {
	var nt NodeToggle
	nt.SetSomeDecl(enabled)
	return nt
}

func EveryLocation(enabled bool) NodeToggle {
	var nt NodeToggle
	nt.SetEvery(enabled)
	return nt
}

func WithLocation(enabled bool) NodeToggle {
	var nt NodeToggle
	nt.SetWith(enabled)
	return nt
}

func AnnotationsLocation(enabled bool) NodeToggle {
	var nt NodeToggle
	nt.SetAnnotations(enabled)
	return nt
}

func AnnotationsRefLocation(enabled bool) NodeToggle {
	var nt NodeToggle
	nt.SetAnnotationsRef(enabled)
	return nt
}

func AllLocations() NodeToggle {
	var nt NodeToggle
	nt.flags = 0xFFFF
	return nt
}
