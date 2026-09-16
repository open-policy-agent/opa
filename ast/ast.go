// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package ast

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	astJSON "github.com/open-policy-agent/opa/ast/json"
	v1 "github.com/open-policy-agent/opa/v1/ast"
)

type (
	// Annotations represents metadata attached to other AST nodes such as rules.
	Annotations = v1.Annotations

	// SchemaAnnotation contains a schema declaration for the document identified by the path.
	SchemaAnnotation = v1.SchemaAnnotation

	AuthorAnnotation = v1.AuthorAnnotation

	RelatedResourceAnnotation = v1.RelatedResourceAnnotation

	AnnotationSet = v1.AnnotationSet

	AnnotationsRef = v1.AnnotationsRef

	AnnotationsRefSet = v1.AnnotationsRefSet

	FlatAnnotationsRefSet = v1.FlatAnnotationsRefSet
)

func NewAnnotationsRef(a *Annotations) *AnnotationsRef {
	return v1.NewAnnotationsRef(a)
}

func BuildAnnotationSet(modules []*Module) (*AnnotationSet, Errors) {
	return v1.BuildAnnotationSet(modules)
}

// Builtins is the registry of built-in functions supported by OPA.
// Call RegisterBuiltin to add a new built-in.
var Builtins = v1.Builtins

// RegisterBuiltin adds a new built-in function to the registry.
func RegisterBuiltin(b *Builtin) {
	v1.RegisterBuiltin(b)
}

// DefaultBuiltins is the registry of built-in functions supported in OPA
// by default. When adding a new built-in function to OPA, update this
// list.
var DefaultBuiltins = v1.DefaultBuiltins

// BuiltinMap provides a convenient mapping of built-in names to
// built-in definitions.
var BuiltinMap = v1.BuiltinMap

// Deprecated: Builtins can now be directly annotated with the
// Nondeterministic property, and when set to true, will be ignored
// for partial evaluation.
var IgnoreDuringPartialEval = v1.IgnoreDuringPartialEval

/**
 * Unification
 */

// Equality represents the "=" operator.
var Equality = v1.Equality

/**
 * Assignment
 */

// Assign represents the assignment (":=") operator.
var Assign = v1.Assign

// Member represents the `in` (infix) operator.
var Member = v1.Member

// MemberWithKey represents the `in` (infix) operator when used
// with two terms on the lhs, i.e., `k, v in obj`.
var MemberWithKey = v1.MemberWithKey

var GreaterThan = v1.GreaterThan

var GreaterThanEq = v1.GreaterThanEq

// LessThan represents the "<" comparison operator.
var LessThan = v1.LessThan

var LessThanEq = v1.LessThanEq

var NotEqual = v1.NotEqual

// Equal represents the "==" comparison operator.
var Equal = v1.Equal

var Plus = v1.Plus

var Minus = v1.Minus

var Multiply = v1.Multiply

var Divide = v1.Divide

var Round = v1.Round

var Ceil = v1.Ceil

var Floor = v1.Floor

var Abs = v1.Abs

var Rem = v1.Rem

/**
 * Bitwise
 */

var BitsOr = v1.BitsOr

var BitsAnd = v1.BitsAnd

var BitsNegate = v1.BitsNegate

var BitsXOr = v1.BitsXOr

var BitsShiftLeft = v1.BitsShiftLeft

var BitsShiftRight = v1.BitsShiftRight

/**
 * Sets
 */

var And = v1.And

// Or performs a union operation on sets.
var Or = v1.Or

var Intersection = v1.Intersection

var Union = v1.Union

/**
 * Aggregates
 */

var Count = v1.Count

var Sum = v1.Sum

var Product = v1.Product

var Max = v1.Max

var Min = v1.Min

/**
 * Sorting
 */

var Sort = v1.Sort

/**
 * Arrays
 */

var ArrayConcat = v1.ArrayConcat

var ArraySlice = v1.ArraySlice

var ArrayReverse = v1.ArrayReverse

/**
 * Conversions
 */

var ToNumber = v1.ToNumber

/**
 * Regular Expressions
 */

var RegexMatch = v1.RegexMatch

var RegexIsValid = v1.RegexIsValid

var RegexFindAllStringSubmatch = v1.RegexFindAllStringSubmatch

var RegexTemplateMatch = v1.RegexTemplateMatch

var RegexSplit = v1.RegexSplit

// RegexFind takes two strings and a number, the pattern, the value and number of match values to
// return, -1 means all match values.
var RegexFind = v1.RegexFind

// GlobsMatch takes two strings regexp-style strings and evaluates to true if their
// intersection matches a non-empty set of non-empty strings.
// Examples:
//   - "a.a." and ".b.b" -> true.
//   - "[a-z]*" and [0-9]+" -> not true.
var GlobsMatch = v1.GlobsMatch

/**
 * Strings
 */

var AnyPrefixMatch = v1.AnyPrefixMatch

var AnySuffixMatch = v1.AnySuffixMatch

var Concat = v1.Concat

var FormatInt = v1.FormatInt

var IndexOf = v1.IndexOf

var IndexOfN = v1.IndexOfN

var Substring = v1.Substring

var Contains = v1.Contains

var StringCount = v1.StringCount

var StartsWith = v1.StartsWith

var EndsWith = v1.EndsWith

var Lower = v1.Lower

var Upper = v1.Upper

var Split = v1.Split

var Replace = v1.Replace

var ReplaceN = v1.ReplaceN

var RegexReplace = v1.RegexReplace

var Trim = v1.Trim

var TrimLeft = v1.TrimLeft

var TrimPrefix = v1.TrimPrefix

var TrimRight = v1.TrimRight

var TrimSuffix = v1.TrimSuffix

var TrimSpace = v1.TrimSpace

var Sprintf = v1.Sprintf

var StringReverse = v1.StringReverse

var RenderTemplate = v1.RenderTemplate

/**
 * Numbers
 */

// RandIntn returns a random number 0 - n
// Marked non-deterministic because it relies on RNG internally.
var RandIntn = v1.RandIntn

var NumbersRange = v1.NumbersRange

var NumbersRangeStep = v1.NumbersRangeStep

/**
 * Units
 */

var UnitsParse = v1.UnitsParse

var UnitsParseBytes = v1.UnitsParseBytes

//
/**
 * Type
 */

// UUIDRFC4122 returns a version 4 UUID string.
// Marked non-deterministic because it relies on RNG internally.
var UUIDRFC4122 = v1.UUIDRFC4122

var UUIDParse = v1.UUIDParse

/**
 * JSON
 */

var JSONFilter = v1.JSONFilter

var JSONRemove = v1.JSONRemove

var JSONPatch = v1.JSONPatch

var ObjectSubset = v1.ObjectSubset

var ObjectUnion = v1.ObjectUnion

var ObjectUnionN = v1.ObjectUnionN

var ObjectRemove = v1.ObjectRemove

var ObjectFilter = v1.ObjectFilter

var ObjectGet = v1.ObjectGet

var ObjectKeys = v1.ObjectKeys

/*
 *  Encoding
 */

var JSONMarshal = v1.JSONMarshal

var JSONMarshalWithOptions = v1.JSONMarshalWithOptions

var JSONUnmarshal = v1.JSONUnmarshal

var JSONIsValid = v1.JSONIsValid

var Base64Encode = v1.Base64Encode

var Base64Decode = v1.Base64Decode

var Base64IsValid = v1.Base64IsValid

var Base64UrlEncode = v1.Base64UrlEncode

var Base64UrlEncodeNoPad = v1.Base64UrlEncodeNoPad

var Base64UrlDecode = v1.Base64UrlDecode

var URLQueryDecode = v1.URLQueryDecode

var URLQueryEncode = v1.URLQueryEncode

var URLQueryEncodeObject = v1.URLQueryEncodeObject

var URLQueryDecodeObject = v1.URLQueryDecodeObject

var YAMLMarshal = v1.YAMLMarshal

var YAMLUnmarshal = v1.YAMLUnmarshal

// YAMLIsValid verifies the input string is a valid YAML document.
var YAMLIsValid = v1.YAMLIsValid

var HexEncode = v1.HexEncode

var HexDecode = v1.HexDecode

/**
 * Tokens
 */

var JWTDecode = v1.JWTDecode

var JWTVerifyRS256 = v1.JWTVerifyRS256

var JWTVerifyRS384 = v1.JWTVerifyRS384

var JWTVerifyRS512 = v1.JWTVerifyRS512

var JWTVerifyPS256 = v1.JWTVerifyPS256

var JWTVerifyPS384 = v1.JWTVerifyPS384

var JWTVerifyPS512 = v1.JWTVerifyPS512

var JWTVerifyES256 = v1.JWTVerifyES256

var JWTVerifyES384 = v1.JWTVerifyES384

var JWTVerifyES512 = v1.JWTVerifyES512

var JWTVerifyHS256 = v1.JWTVerifyHS256

var JWTVerifyHS384 = v1.JWTVerifyHS384

var JWTVerifyHS512 = v1.JWTVerifyHS512

// Marked non-deterministic because it relies on time internally.
var JWTDecodeVerify = v1.JWTDecodeVerify

// Marked non-deterministic because it relies on RNG internally.
var JWTEncodeSignRaw = v1.JWTEncodeSignRaw

// Marked non-deterministic because it relies on RNG internally.
var JWTEncodeSign = v1.JWTEncodeSign

/**
 * Time
 */

// Marked non-deterministic because it relies on time directly.
var NowNanos = v1.NowNanos

var ParseNanos = v1.ParseNanos

var ParseRFC3339Nanos = v1.ParseRFC3339Nanos

var ParseDurationNanos = v1.ParseDurationNanos

var Format = v1.Format

var Date = v1.Date

var Clock = v1.Clock

var Weekday = v1.Weekday

var AddDate = v1.AddDate

var Diff = v1.Diff

/**
 * Crypto.
 */

var CryptoX509ParseCertificates = v1.CryptoX509ParseCertificates

var CryptoX509ParseAndVerifyCertificates = v1.CryptoX509ParseAndVerifyCertificates

var CryptoX509ParseAndVerifyCertificatesWithOptions = v1.CryptoX509ParseAndVerifyCertificatesWithOptions

var CryptoX509ParseCertificateRequest = v1.CryptoX509ParseCertificateRequest

var CryptoX509ParseKeyPair = v1.CryptoX509ParseKeyPair
var CryptoX509ParseRSAPrivateKey = v1.CryptoX509ParseRSAPrivateKey

var CryptoParsePrivateKeys = v1.CryptoParsePrivateKeys

var CryptoMd5 = v1.CryptoMd5

var CryptoSha1 = v1.CryptoSha1

var CryptoSha256 = v1.CryptoSha256

var CryptoHmacMd5 = v1.CryptoHmacMd5

var CryptoHmacSha1 = v1.CryptoHmacSha1

var CryptoHmacSha256 = v1.CryptoHmacSha256

var CryptoHmacSha512 = v1.CryptoHmacSha512

var CryptoHmacEqual = v1.CryptoHmacEqual

/**
 * Graphs.
 */

var WalkBuiltin = v1.WalkBuiltin

var ReachableBuiltin = v1.ReachableBuiltin

var ReachablePathsBuiltin = v1.ReachablePathsBuiltin

/**
 * Type
 */

var IsNumber = v1.IsNumber

var IsString = v1.IsString

var IsBoolean = v1.IsBoolean

var IsArray = v1.IsArray

var IsSet = v1.IsSet

var IsObject = v1.IsObject

var IsNull = v1.IsNull

/**
 * Type Name
 */

// TypeNameBuiltin returns the type of the input.
var TypeNameBuiltin = v1.TypeNameBuiltin

/**
 * HTTP Request
 */

// Marked non-deterministic because HTTP request results can be non-deterministic.
var HTTPSend = v1.HTTPSend

/**
 * GraphQL
 */

// GraphQLParse returns a pair of AST objects from parsing/validation.
var GraphQLParse = v1.GraphQLParse

// GraphQLParseAndVerify returns a boolean and a pair of AST object from parsing/validation.
var GraphQLParseAndVerify = v1.GraphQLParseAndVerify

// GraphQLParseQuery parses the input GraphQL query and returns a JSON
// representation of its AST.
var GraphQLParseQuery = v1.GraphQLParseQuery

// GraphQLParseSchema parses the input GraphQL schema and returns a JSON
// representation of its AST.
var GraphQLParseSchema = v1.GraphQLParseSchema

// GraphQLIsValid returns true if a GraphQL query is valid with a given
// schema, and returns false for all other inputs.
var GraphQLIsValid = v1.GraphQLIsValid

// GraphQLSchemaIsValid returns true if the input is valid GraphQL schema,
// and returns false for all other inputs.
var GraphQLSchemaIsValid = v1.GraphQLSchemaIsValid

/**
 * JSON Schema
 */

// JSONSchemaVerify returns empty string if the input is valid JSON schema
// and returns error string for all other inputs.
var JSONSchemaVerify = v1.JSONSchemaVerify

// JSONMatchSchema returns empty array if the document matches the JSON schema,
// and returns non-empty array with error objects otherwise.
var JSONMatchSchema = v1.JSONMatchSchema

/**
 * Cloud Provider Helper Functions
 */

var ProvidersAWSSignReqObj = v1.ProvidersAWSSignReqObj

/**
 * Rego
 */

var RegoParseModule = v1.RegoParseModule

var RegoMetadataChain = v1.RegoMetadataChain

// RegoMetadataRule returns the metadata for the active rule
var RegoMetadataRule = v1.RegoMetadataRule

/**
 * OPA
 */

// Marked non-deterministic because of unpredictable config/environment-dependent results.
var OPARuntime = v1.OPARuntime

/**
 * Trace
 */

var Trace = v1.Trace

/**
 * Glob
 */

var GlobMatch = v1.GlobMatch

var GlobQuoteMeta = v1.GlobQuoteMeta

/**
 * Networking
 */

var NetCIDRIntersects = v1.NetCIDRIntersects

var NetCIDRExpand = v1.NetCIDRExpand

var NetCIDRContains = v1.NetCIDRContains

var NetCIDRContainsMatches = v1.NetCIDRContainsMatches

var NetCIDRMerge = v1.NetCIDRMerge

var NetCIDRIsValid = v1.NetCIDRIsValid

// Marked non-deterministic because DNS resolution results can be non-deterministic.
var NetLookupIPAddr = v1.NetLookupIPAddr

/**
 * Semantic Versions
 */

var SemVerIsValid = v1.SemVerIsValid

var SemVerCompare = v1.SemVerCompare

/**
 * Printing
 */

// Print is a special built-in function that writes zero or more operands
// to a message buffer. The caller controls how the buffer is displayed. The
// operands may be of any type. Furthermore, unlike other built-in functions,
// undefined operands DO NOT cause the print() function to fail during
// evaluation.
var Print = v1.Print

// InternalPrint represents the internal implementation of the print() function.
// The compiler rewrites print() calls to refer to the internal implementation.
var InternalPrint = v1.InternalPrint

/**
 * Deprecated built-ins.
 */

// SetDiff has been replaced by the minus built-in.
var SetDiff = v1.SetDiff

// NetCIDROverlap has been replaced by the `net.cidr_contains` built-in.
var NetCIDROverlap = v1.NetCIDROverlap

// CastArray checks the underlying type of the input. If it is array or set, an array
// containing the values is returned. If it is not an array, an error is thrown.
var CastArray = v1.CastArray

// CastSet checks the underlying type of the input.
// If it is a set, the set is returned.
// If it is an array, the array is returned in set form (all duplicates removed)
// If neither, an error is thrown
var CastSet = v1.CastSet

// CastString returns input if it is a string; if not returns error.
// For formatting variables, see sprintf
var CastString = v1.CastString

// CastBoolean returns input if it is a boolean; if not returns error.
var CastBoolean = v1.CastBoolean

// CastNull returns null if input is null; if not returns error.
var CastNull = v1.CastNull

// CastObject returns the given object if it is null; throws an error otherwise
var CastObject = v1.CastObject

// RegexMatchDeprecated declares `re_match` which has been deprecated. Use `regex.match` instead.
var RegexMatchDeprecated = v1.RegexMatchDeprecated

// All takes a list and returns true if all of the items
// are true. A collection of length 0 returns true.
var All = v1.All

// Any takes a collection and returns true if any of the items
// is true. A collection of length 0 returns false.
var Any = v1.Any

// Builtin represents a built-in function supported by OPA. Every built-in
// function is uniquely identified by a name.
type Builtin = v1.Builtin

// VersonIndex contains an index from built-in function name, language feature,
// and future rego keyword to version number. During the build, this is used to
// create an index of the minimum version required for the built-in/feature/kw.
type VersionIndex = v1.VersionIndex

// In the compiler, we used this to check that we're OK working with ref heads.
// If this isn't present, we'll fail. This is to ensure that older versions of
// OPA can work with policies that we're compiling -- if they don't know ref
// heads, they wouldn't be able to parse them.
const FeatureRefHeadStringPrefixes = v1.FeatureRefHeadStringPrefixes
const FeatureRefHeads = v1.FeatureRefHeads
const FeatureRegoV1 = v1.FeatureRegoV1
const FeatureRegoV1Import = v1.FeatureRegoV1Import

// Capabilities defines a structure containing data that describes the capabilities
// or features supported by a particular version of OPA.
type Capabilities = v1.Capabilities

// WasmABIVersion captures the Wasm ABI version. Its `Minor` version is indicating
// backwards-compatible changes.
type WasmABIVersion = v1.WasmABIVersion

// CapabilitiesForThisVersion returns the capabilities of this version of OPA.
func CapabilitiesForThisVersion() *Capabilities {
	return v1.CapabilitiesForThisVersion(v1.CapabilitiesRegoVersion(DefaultRegoVersion))
}

// LoadCapabilitiesJSON loads a JSON serialized capabilities structure from the reader r.
func LoadCapabilitiesJSON(r io.Reader) (*Capabilities, error) {
	return v1.LoadCapabilitiesJSON(r)
}

// LoadCapabilitiesVersion loads a JSON serialized capabilities structure from the specific version.
func LoadCapabilitiesVersion(version string) (*Capabilities, error) {
	return v1.LoadCapabilitiesVersion(version)
}

// LoadCapabilitiesFile loads a JSON serialized capabilities structure from a file.
func LoadCapabilitiesFile(file string) (*Capabilities, error) {
	return v1.LoadCapabilitiesFile(file)
}

// LoadCapabilitiesVersions loads all capabilities versions
func LoadCapabilitiesVersions() ([]string, error) {
	return v1.LoadCapabilitiesVersions()
}

// UnificationErrDetail describes a type mismatch error when two values are
// unified (e.g., x = [1,2,y]).
type UnificationErrDetail = v1.UnificationErrDetail

// RefErrUnsupportedDetail describes an undefined reference error where the
// referenced value does not support dereferencing (e.g., scalars).
type RefErrUnsupportedDetail = v1.RefErrUnsupportedDetail

// RefErrInvalidDetail describes an undefined reference error where the referenced
// value does not support the reference operand (e.g., missing object key,
// invalid key type, etc.)
type RefErrInvalidDetail = v1.RefErrInvalidDetail

// Compare returns an integer indicating whether two AST values are less than,
// equal to, or greater than each other.
//
// If a is less than b, the return value is negative. If a is greater than b,
// the return value is positive. If a is equal to b, the return value is zero.
//
// Different types are never equal to each other. For comparison purposes, types
// are sorted as follows:
//
// nil < Null < Boolean < Number < String < Var < Ref < Array < Object < Set <
// ArrayComprehension < ObjectComprehension < SetComprehension < Expr < SomeDecl
// < With < Body < Rule < Import < Package < Module.
//
// Arrays and Refs are equal if and only if both a and b have the same length
// and all corresponding elements are equal. If one element is not equal, the
// return value is the same as for the first differing element. If all elements
// are equal but a and b have different lengths, the shorter is considered less
// than the other.
//
// Objects are considered equal if and only if both a and b have the same sorted
// (key, value) pairs and are of the same length. Other comparisons are
// consistent but not defined.
//
// Sets are considered equal if and only if the symmetric difference of a and b
// is empty.
// Other comparisons are consistent but not defined.
func Compare(a, b any) int {
	return v1.Compare(a, b)
}

// CompileErrorLimitDefault is the default number errors a compiler will allow before
// exiting.
const CompileErrorLimitDefault = 10

// Compiler contains the state of a compilation process.
type Compiler = v1.Compiler

// CompilerStage defines the interface for stages in the compiler.
type CompilerStage = v1.CompilerStage

// CompilerEvalMode allows toggling certain stages that are only
// needed for certain modes, Concretely, only "topdown" mode will
// have the compiler build comprehension and rule indices.
type CompilerEvalMode = v1.CompilerEvalMode

const (
	// EvalModeTopdown (default) instructs the compiler to build rule
	// and comprehension indices used by topdown evaluation.
	EvalModeTopdown = v1.EvalModeTopdown

	// EvalModeIR makes the compiler skip the stages for comprehension
	// and rule indices.
	EvalModeIR = v1.EvalModeIR
)

// CompilerStageDefinition defines a compiler stage
type CompilerStageDefinition = v1.CompilerStageDefinition

// RulesOptions defines the options for retrieving rules by Ref from the
// compiler.
type RulesOptions = v1.RulesOptions

// QueryContext contains contextual information for running an ad-hoc query.
//
// Ad-hoc queries can be run in the context of a package and imports may be
// included to provide concise access to data.
type QueryContext = v1.QueryContext

// NewQueryContext returns a new QueryContext object.
func NewQueryContext() *QueryContext {
	return v1.NewQueryContext()
}

// QueryCompiler defines the interface for compiling ad-hoc queries.
type QueryCompiler = v1.QueryCompiler

// QueryCompilerStage defines the interface for stages in the query compiler.
type QueryCompilerStage = v1.QueryCompilerStage

// QueryCompilerStageDefinition defines a QueryCompiler stage
type QueryCompilerStageDefinition = v1.QueryCompilerStageDefinition

// NewCompiler returns a new empty compiler.
func NewCompiler() *Compiler {
	return v1.NewCompiler().WithDefaultRegoVersion(DefaultRegoVersion)
}

// ModuleLoader defines the interface that callers can implement to enable lazy
// loading of modules during compilation.
type ModuleLoader = v1.ModuleLoader

// SafetyCheckVisitorParams defines the AST visitor parameters to use for collecting
// variables during the safety check. This has to be exported because it's relied on
// by the copy propagation implementation in topdown.
var SafetyCheckVisitorParams = v1.SafetyCheckVisitorParams

// ComprehensionIndex specifies how the comprehension term can be indexed. The keys
// tell the evaluator what variables to use for indexing. In the future, the index
// could be expanded with more information that would allow the evaluator to index
// a larger fragment of comprehensions (e.g., by closing over variables in the outer
// query.)
type ComprehensionIndex = v1.ComprehensionIndex

// ModuleTreeNode represents a node in the module tree. The module
// tree is keyed by the package path.
type ModuleTreeNode = v1.ModuleTreeNode

// TreeNode represents a node in the rule tree. The rule tree is keyed by
// rule path.
type TreeNode = v1.TreeNode

// NewRuleTree returns a new TreeNode that represents the root
// of the rule tree populated with the given rules.
func NewRuleTree(mtree *ModuleTreeNode) *TreeNode {
	return v1.NewRuleTree(mtree)
}

// Graph represents the graph of dependencies between rules.
type Graph = v1.Graph

// NewGraph returns a new Graph based on modules. The list function must return
// the rules referred to directly by the ref.
func NewGraph(modules map[string]*Module, list func(Ref) []*Rule) *Graph {
	return v1.NewGraph(modules, list)
}

// GraphTraversal is a Traversal that understands the dependency graph
type GraphTraversal = v1.GraphTraversal

// NewGraphTraversal returns a Traversal for the dependency graph
func NewGraphTraversal(graph *Graph) *GraphTraversal {
	return v1.NewGraphTraversal(graph)
}

// OutputVarsFromBody returns all variables which are the "output" for
// the given body. For safety checks this means that they would be
// made safe by the body.
func OutputVarsFromBody(c *Compiler, body Body, safe VarSet) VarSet {
	return v1.OutputVarsFromBody(c, body, safe)
}

// OutputVarsFromExpr returns all variables which are the "output" for
// the given expression. For safety checks this means that they would be
// made safe by the expr.
func OutputVarsFromExpr(c *Compiler, expr *Expr, safe VarSet) VarSet {
	return v1.OutputVarsFromExpr(c, expr, safe)
}

// CompileModules takes a set of Rego modules represented as strings and
// compiles them for evaluation. The keys of the map are used as filenames.
func CompileModules(modules map[string]string) (*Compiler, error) {
	return CompileModulesWithOpt(modules, CompileOpts{
		ParserOptions: ParserOptions{
			RegoVersion: DefaultRegoVersion,
		},
	})
}

// CompileOpts defines a set of options for the compiler.
type CompileOpts = v1.CompileOpts

// CompileModulesWithOpt takes a set of Rego modules represented as strings and
// compiles them for evaluation. The keys of the map are used as filenames.
func CompileModulesWithOpt(modules map[string]string, opts CompileOpts) (*Compiler, error) {
	if opts.ParserOptions.RegoVersion == RegoUndefined {
		opts.ParserOptions.RegoVersion = DefaultRegoVersion
	}

	return v1.CompileModulesWithOpt(modules, opts)
}

// MustCompileModules compiles a set of Rego modules represented as strings. If
// the compilation process fails, this function panics.
func MustCompileModules(modules map[string]string) *Compiler {
	return MustCompileModulesWithOpts(modules, CompileOpts{})
}

// MustCompileModulesWithOpts compiles a set of Rego modules represented as strings. If
// the compilation process fails, this function panics.
func MustCompileModulesWithOpts(modules map[string]string, opts CompileOpts) *Compiler {

	compiler, err := CompileModulesWithOpt(modules, opts)
	if err != nil {
		panic(err)
	}

	return compiler
}

// CheckPathConflicts returns a set of errors indicating paths that
// are in conflict with the result of the provided callable.
func CheckPathConflicts(c *Compiler, exists func([]string) (bool, error)) Errors {
	return v1.CheckPathConflicts(c, exists)
}

// TypeEnv contains type info for static analysis such as type checking.
type TypeEnv = v1.TypeEnv

// Errors represents a series of errors encountered during parsing, compiling,
// etc.
type Errors = v1.Errors

const (
	// ParseErr indicates an unclassified parse error occurred.
	ParseErr = v1.ParseErr

	// CompileErr indicates an unclassified compile error occurred.
	CompileErr = v1.CompileErr

	// TypeErr indicates a type error was caught.
	TypeErr = v1.TypeErr

	// UnsafeVarErr indicates an unsafe variable was found during compilation.
	UnsafeVarErr = v1.UnsafeVarErr

	// RecursionErr indicates recursion was found during compilation.
	RecursionErr = v1.RecursionErr
)

// IsError returns true if err is an AST error with code.
func IsError(code string, err error) bool {
	return v1.IsError(code, err)
}

// ErrorDetails defines the interface for detailed error messages.
type ErrorDetails = v1.ErrorDetails

// Error represents a single error caught during parsing, compiling, etc.
type Error = v1.Error

// NewError returns a new Error object.
func NewError(code string, loc *Location, f string, a ...any) *Error {
	return v1.NewError(code, loc, f, a...)
}

// RuleIndex defines the interface for rule indices.
type RuleIndex v1.RuleIndex

// IndexResult contains the result of an index lookup.
type IndexResult = v1.IndexResult

// NewIndexResult returns a new IndexResult object.
func NewIndexResult(kind RuleKind) *IndexResult {
	return v1.NewIndexResult(kind)
}

func InternedBooleanTerm(b bool) *Term {
	return v1.InternedTerm(b)
}

// InternedIntNumberTerm returns a term with the given integer value. The term is
// cached between -1 to 512, and for values outside of that range, this function
// is equivalent to ast.IntNumberTerm.
func InternedIntNumberTerm(i int) *Term {
	return v1.InternedTerm(i)
}

func HasInternedIntNumberTerm(i int) bool {
	return v1.HasInternedIntNumberTerm(i)
}

// ValueMap represents a key/value map between AST term values. Any type of term
// can be used as a key in the map.
type ValueMap = v1.ValueMap

// NewValueMap returns a new ValueMap.
func NewValueMap() *ValueMap {
	return v1.NewValueMap()
}

var RegoV1CompatibleRef = v1.RegoV1CompatibleRef

// RegoVersion defines the Rego syntax requirements for a module.
type RegoVersion = v1.RegoVersion

const DefaultRegoVersion = RegoV0

const (
	RegoUndefined = v1.RegoUndefined
	// RegoV0 is the default, original Rego syntax.
	RegoV0 = v1.RegoV0
	// RegoV0CompatV1 requires modules to comply with both the RegoV0 and RegoV1 syntax (as when 'rego.v1' is imported in a module).
	// Shortly, RegoV1 compatibility is required, but 'rego.v1' or 'future.keywords' must also be imported.
	RegoV0CompatV1 = v1.RegoV0CompatV1
	// RegoV1 is the Rego syntax enforced by OPA 1.0; e.g.:
	// future.keywords part of default keyword set, and don't require imports;
	// 'if' and 'contains' required in rule heads;
	// (some) strict checks on by default.
	RegoV1 = v1.RegoV1
)

func RegoVersionFromInt(i int) RegoVersion {
	return v1.RegoVersionFromInt(i)
}

// Parser is used to parse Rego statements.
type Parser = v1.Parser

// ParserOptions defines the options for parsing Rego statements.
type ParserOptions = v1.ParserOptions

// NewParser creates and initializes a Parser.
func NewParser() *Parser {
	return v1.NewParser().WithRegoVersion(DefaultRegoVersion)
}

func IsFutureKeyword(s string) bool {
	return v1.IsFutureKeywordForRegoVersion(s, RegoV0)
}

// MustParseBody returns a parsed body.
// If an error occurs during parsing, panic.
func MustParseBody(input string) Body {
	return MustParseBodyWithOpts(input, ParserOptions{})
}

// MustParseBodyWithOpts returns a parsed body.
// If an error occurs during parsing, panic.
func MustParseBodyWithOpts(input string, opts ParserOptions) Body {
	return v1.MustParseBodyWithOpts(input, setDefaultRegoVersion(opts))
}

// MustParseExpr returns a parsed expression.
// If an error occurs during parsing, panic.
func MustParseExpr(input string) *Expr {
	parsed, err := ParseExpr(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseImports returns a slice of imports.
// If an error occurs during parsing, panic.
func MustParseImports(input string) []*Import {
	parsed, err := ParseImports(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseModule returns a parsed module.
// If an error occurs during parsing, panic.
func MustParseModule(input string) *Module {
	return MustParseModuleWithOpts(input, ParserOptions{})
}

// MustParseModuleWithOpts returns a parsed module.
// If an error occurs during parsing, panic.
func MustParseModuleWithOpts(input string, opts ParserOptions) *Module {
	return v1.MustParseModuleWithOpts(input, setDefaultRegoVersion(opts))
}

// MustParsePackage returns a Package.
// If an error occurs during parsing, panic.
func MustParsePackage(input string) *Package {
	parsed, err := ParsePackage(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseStatements returns a slice of parsed statements.
// If an error occurs during parsing, panic.
func MustParseStatements(input string) []Statement {
	parsed, _, err := ParseStatements("", input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseStatement returns exactly one statement.
// If an error occurs during parsing, panic.
func MustParseStatement(input string) Statement {
	parsed, err := ParseStatement(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

func MustParseStatementWithOpts(input string, popts ParserOptions) Statement {
	return v1.MustParseStatementWithOpts(input, setDefaultRegoVersion(popts))
}

// MustParseRef returns a parsed reference.
// If an error occurs during parsing, panic.
func MustParseRef(input string) Ref {
	parsed, err := ParseRef(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseRule returns a parsed rule.
// If an error occurs during parsing, panic.
func MustParseRule(input string) *Rule {
	parsed, err := ParseRule(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// MustParseRuleWithOpts returns a parsed rule.
// If an error occurs during parsing, panic.
func MustParseRuleWithOpts(input string, opts ParserOptions) *Rule {
	return v1.MustParseRuleWithOpts(input, setDefaultRegoVersion(opts))
}

// MustParseTerm returns a parsed term.
// If an error occurs during parsing, panic.
func MustParseTerm(input string) *Term {
	parsed, err := ParseTerm(input)
	if err != nil {
		panic(err)
	}
	return parsed
}

// ParseRuleFromBody returns a rule if the body can be interpreted as a rule
// definition. Otherwise, an error is returned.
func ParseRuleFromBody(module *Module, body Body) (*Rule, error) {
	return v1.ParseRuleFromBody(module, body)
}

// ParseRuleFromExpr returns a rule if the expression can be interpreted as a
// rule definition.
func ParseRuleFromExpr(module *Module, expr *Expr) (*Rule, error) {
	return v1.ParseRuleFromExpr(module, expr)
}

// ParseCompleteDocRuleFromAssignmentExpr returns a rule if the expression can
// be interpreted as a complete document definition declared with the assignment
// operator.
func ParseCompleteDocRuleFromAssignmentExpr(module *Module, lhs, rhs *Term) (*Rule, error) {
	return v1.ParseCompleteDocRuleFromAssignmentExpr(module, lhs, rhs)
}

// ParseCompleteDocRuleFromEqExpr returns a rule if the expression can be
// interpreted as a complete document definition.
func ParseCompleteDocRuleFromEqExpr(module *Module, lhs, rhs *Term) (*Rule, error) {
	return v1.ParseCompleteDocRuleFromEqExpr(module, lhs, rhs)
}

func ParseCompleteDocRuleWithDotsFromTerm(module *Module, term *Term) (*Rule, error) {
	return v1.ParseCompleteDocRuleWithDotsFromTerm(module, term)
}

// ParsePartialObjectDocRuleFromEqExpr returns a rule if the expression can be
// interpreted as a partial object document definition.
func ParsePartialObjectDocRuleFromEqExpr(module *Module, lhs, rhs *Term) (*Rule, error) {
	return v1.ParsePartialObjectDocRuleFromEqExpr(module, lhs, rhs)
}

// ParsePartialSetDocRuleFromTerm returns a rule if the term can be interpreted
// as a partial set document definition.
func ParsePartialSetDocRuleFromTerm(module *Module, term *Term) (*Rule, error) {
	return v1.ParsePartialSetDocRuleFromTerm(module, term)
}

// ParseRuleFromCallEqExpr returns a rule if the term can be interpreted as a
// function definition (e.g., f(x) = y => f(x) = y { true }).
func ParseRuleFromCallEqExpr(module *Module, lhs, rhs *Term) (*Rule, error) {
	return v1.ParseRuleFromCallEqExpr(module, lhs, rhs)
}

// ParseRuleFromCallExpr returns a rule if the terms can be interpreted as a
// function returning true or some value (e.g., f(x) => f(x) = true { true }).
func ParseRuleFromCallExpr(module *Module, terms []*Term) (*Rule, error) {
	return v1.ParseRuleFromCallExpr(module, terms)
}

// ParseImports returns a slice of Import objects.
func ParseImports(input string) ([]*Import, error) {
	return v1.ParseImports(input)
}

// ParseModule returns a parsed Module object.
// For details on Module objects and their fields, see policy.go.
// Empty input will return nil, nil.
func ParseModule(filename, input string) (*Module, error) {
	return ParseModuleWithOpts(filename, input, ParserOptions{})
}

// ParseModuleWithOpts returns a parsed Module object, and has an additional input ParserOptions
// For details on Module objects and their fields, see policy.go.
// Empty input will return nil, nil.
func ParseModuleWithOpts(filename, input string, popts ParserOptions) (*Module, error) {
	return v1.ParseModuleWithOpts(filename, input, setDefaultRegoVersion(popts))
}

// ParseBody returns exactly one body.
// If multiple bodies are parsed, an error is returned.
func ParseBody(input string) (Body, error) {
	return ParseBodyWithOpts(input, ParserOptions{SkipRules: true})
}

// ParseBodyWithOpts returns exactly one body. It does _not_ set SkipRules: true on its own,
// but respects whatever ParserOptions it's been given.
func ParseBodyWithOpts(input string, popts ParserOptions) (Body, error) {
	return v1.ParseBodyWithOpts(input, setDefaultRegoVersion(popts))
}

// ParseExpr returns exactly one expression.
// If multiple expressions are parsed, an error is returned.
func ParseExpr(input string) (*Expr, error) {
	body, err := ParseBody(input)
	if err != nil {
		return nil, fmt.Errorf("failed to parse expression: %w", err)
	}
	if len(body) != 1 {
		return nil, fmt.Errorf("expected exactly one expression but got: %v", body)
	}
	return body[0], nil
}

// ParsePackage returns exactly one Package.
// If multiple statements are parsed, an error is returned.
func ParsePackage(input string) (*Package, error) {
	return v1.ParsePackage(input)
}

// ParseTerm returns exactly one term.
// If multiple terms are parsed, an error is returned.
func ParseTerm(input string) (*Term, error) {
	body, err := ParseBody(input)
	if err != nil {
		return nil, fmt.Errorf("failed to parse term: %w", err)
	}
	if len(body) != 1 {
		return nil, fmt.Errorf("expected exactly one term but got: %v", body)
	}
	term, ok := body[0].Terms.(*Term)
	if !ok {
		return nil, fmt.Errorf("expected term but got %v", body[0].Terms)
	}
	return term, nil
}

// ParseRef returns exactly one reference.
func ParseRef(input string) (Ref, error) {
	term, err := ParseTerm(input)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ref: %w", err)
	}
	ref, ok := term.Value.(Ref)
	if !ok {
		return nil, fmt.Errorf("expected ref but got %v", term)
	}
	return ref, nil
}

// ParseRuleWithOpts returns exactly one rule.
// If multiple rules are parsed, an error is returned.
func ParseRuleWithOpts(input string, opts ParserOptions) (*Rule, error) {
	return v1.ParseRuleWithOpts(input, setDefaultRegoVersion(opts))
}

// ParseRule returns exactly one rule.
// If multiple rules are parsed, an error is returned.
func ParseRule(input string) (*Rule, error) {
	return ParseRuleWithOpts(input, ParserOptions{})
}

// ParseStatement returns exactly one statement.
// A statement might be a term, expression, rule, etc. Regardless,
// this function expects *exactly* one statement. If multiple
// statements are parsed, an error is returned.
func ParseStatement(input string) (Statement, error) {
	stmts, _, err := ParseStatements("", input)
	if err != nil {
		return nil, err
	}
	if len(stmts) != 1 {
		return nil, errors.New("expected exactly one statement")
	}
	return stmts[0], nil
}

func ParseStatementWithOpts(input string, popts ParserOptions) (Statement, error) {
	return v1.ParseStatementWithOpts(input, setDefaultRegoVersion(popts))
}

// ParseStatements is deprecated. Use ParseStatementWithOpts instead.
func ParseStatements(filename, input string) ([]Statement, []*Comment, error) {
	return ParseStatementsWithOpts(filename, input, ParserOptions{})
}

// ParseStatementsWithOpts returns a slice of parsed statements. This is the
// default return value from the parser.
func ParseStatementsWithOpts(filename, input string, popts ParserOptions) ([]Statement, []*Comment, error) {
	return v1.ParseStatementsWithOpts(filename, input, setDefaultRegoVersion(popts))
}

// ParserErrorDetail holds additional details for parser errors.
type ParserErrorDetail = v1.ParserErrorDetail

func setDefaultRegoVersion(opts ParserOptions) ParserOptions {
	if opts.RegoVersion == RegoUndefined {
		opts.RegoVersion = DefaultRegoVersion
	}
	return opts
}

// DefaultRootDocument is the default root document.
//
// All package directives inside source files are implicitly prefixed with the
// DefaultRootDocument value.
var DefaultRootDocument = v1.DefaultRootDocument

// InputRootDocument names the document containing query arguments.
var InputRootDocument = v1.InputRootDocument

// SchemaRootDocument names the document containing external data schemas.
var SchemaRootDocument = v1.SchemaRootDocument

// FunctionArgRootDocument names the document containing function arguments.
// It's only for internal usage, for referencing function arguments between
// the index and topdown.
var FunctionArgRootDocument = v1.FunctionArgRootDocument

// FutureRootDocument names the document containing new, to-become-default,
// features.
var FutureRootDocument = v1.FutureRootDocument

// RegoRootDocument names the document containing new, to-become-default,
// features in a future versioned release.
var RegoRootDocument = v1.RegoRootDocument

// RootDocumentNames contains the names of top-level documents that can be
// referred to in modules and queries.
//
// Note, the schema document is not currently implemented in the evaluator so it
// is not registered as a root document name (yet).
var RootDocumentNames = v1.RootDocumentNames

// DefaultRootRef is a reference to the root of the default document.
//
// All refs to data in the policy engine's storage layer are prefixed with this ref.
var DefaultRootRef = v1.DefaultRootRef

// InputRootRef is a reference to the root of the input document.
//
// All refs to query arguments are prefixed with this ref.
var InputRootRef = v1.InputRootRef

// SchemaRootRef is a reference to the root of the schema document.
//
// All refs to schema documents are prefixed with this ref. Note, the schema
// document is not currently implemented in the evaluator so it is not
// registered as a root document ref (yet).
var SchemaRootRef = v1.SchemaRootRef

// RootDocumentRefs contains the prefixes of top-level documents that all
// non-local references start with.
var RootDocumentRefs = v1.RootDocumentRefs

// SystemDocumentKey is the name of the top-level key that identifies the system
// document.
const SystemDocumentKey = v1.SystemDocumentKey

// ReservedVars is the set of names that refer to implicitly ground vars.
var ReservedVars = v1.ReservedVars

// Wildcard represents the wildcard variable as defined in the language.
var Wildcard = v1.Wildcard

// WildcardPrefix is the special character that all wildcard variables are
// prefixed with when the statement they are contained in is parsed.
const WildcardPrefix = v1.WildcardPrefix

// Keywords contains strings that map to language keywords.
var Keywords = v1.Keywords

var KeywordsV0 = v1.KeywordsV0

var KeywordsV1 = v1.KeywordsV1

func KeywordsForRegoVersion(v RegoVersion) []string {
	return v1.KeywordsForRegoVersion(v)
}

// IsKeyword returns true if s is a language keyword.
func IsKeyword(s string) bool {
	return v1.IsKeyword(s)
}

func IsInKeywords(s string, keywords []string) bool {
	return v1.IsInKeywords(s, keywords)
}

// IsKeywordInRegoVersion returns true if s is a language keyword.
func IsKeywordInRegoVersion(s string, regoVersion RegoVersion) bool {
	return v1.IsKeywordInRegoVersion(s, regoVersion)
}

type (
	// Node represents a node in an AST. Nodes may be statements in a policy module
	// or elements of an ad-hoc query, expression, etc.
	Node = v1.Node

	// Statement represents a single statement in a policy module.
	Statement = v1.Statement
)

type (

	// Module represents a collection of policies (defined by rules)
	// within a namespace (defined by the package) and optional
	// dependencies on external documents (defined by imports).
	Module = v1.Module

	// Comment contains the raw text from the comment in the definition.
	Comment = v1.Comment

	// Package represents the namespace of the documents produced
	// by rules inside the module.
	Package = v1.Package

	// Import represents a dependency on a document outside of the policy
	// namespace. Imports are optional.
	Import = v1.Import

	// Rule represents a rule as defined in the language. Rules define the
	// content of documents that represent policy decisions.
	Rule = v1.Rule

	// Head represents the head of a rule.
	Head = v1.Head

	// Args represents zero or more arguments to a rule.
	Args = v1.Args

	// Body represents one or more expressions contained inside a rule or user
	// function.
	Body = v1.Body

	// Expr represents a single expression contained inside the body of a rule.
	Expr = v1.Expr

	// SomeDecl represents a variable declaration statement. The symbols are variables.
	SomeDecl = v1.SomeDecl

	Every = v1.Every

	// With represents a modifier on an expression.
	With = v1.With
)

// NewComment returns a new Comment object.
func NewComment(text []byte) *Comment {
	return v1.NewComment(text)
}

// IsValidImportPath returns an error indicating if the import path is invalid.
// If the import path is valid, err is nil.
func IsValidImportPath(v Value) (err error) {
	return v1.IsValidImportPath(v)
}

// NewHead returns a new Head object. If args are provided, the first will be
// used for the key and the second will be used for the value.
func NewHead(name Var, args ...*Term) *Head {
	return v1.NewHead(name, args...)
}

// VarHead creates a head object, initializes its Name, Location, and Options,
// and returns the new head.
func VarHead(name Var, location *Location, jsonOpts *astJSON.Options) *Head {
	return v1.VarHead(name, location, jsonOpts)
}

// RefHead returns a new Head object with the passed Ref. If args are provided,
// the first will be used for the value.
func RefHead(ref Ref, args ...*Term) *Head {
	return v1.RefHead(ref, args...)
}

// DocKind represents the collection of document types that can be produced by rules.
type DocKind = v1.DocKind

const (
	// CompleteDoc represents a document that is completely defined by the rule.
	CompleteDoc = v1.CompleteDoc

	// PartialSetDoc represents a set document that is partially defined by the rule.
	PartialSetDoc = v1.PartialSetDoc

	// PartialObjectDoc represents an object document that is partially defined by the rule.
	PartialObjectDoc = v1.PartialObjectDoc
)

type RuleKind = v1.RuleKind

const (
	SingleValue = v1.SingleValue
	MultiValue  = v1.MultiValue
)

// NewBody returns a new Body containing the given expressions. The indices of
// the immediate expressions will be reset.
func NewBody(exprs ...*Expr) Body {
	return v1.NewBody(exprs...)
}

// NewExpr returns a new Expr object.
func NewExpr(terms any) *Expr {
	return v1.NewExpr(terms)
}

// NewBuiltinExpr creates a new Expr object with the supplied terms.
// The builtin operator must be the first term.
func NewBuiltinExpr(terms ...*Term) *Expr {
	return v1.NewBuiltinExpr(terms...)
}

// Copy returns a deep copy of the AST node x. If x is not an AST node, x is returned unmodified.
func Copy(x any) any {
	return v1.Copy(x)
}

// RuleSet represents a collection of rules that produce a virtual document.
type RuleSet = v1.RuleSet

// NewRuleSet returns a new RuleSet containing the given rules.
func NewRuleSet(rules ...*Rule) RuleSet {
	return v1.NewRuleSet(rules...)
}

// Pretty writes a pretty representation of the AST rooted at x to w.
//
// This is function is intended for debug purposes when inspecting ASTs.
func Pretty(w io.Writer, x any) {
	v1.Pretty(w, x)
}

// SchemaSet holds a map from a path to a schema.
type SchemaSet = v1.SchemaSet

// NewSchemaSet returns an empty SchemaSet.
func NewSchemaSet() *SchemaSet {
	return v1.NewSchemaSet()
}

// TypeName returns a human readable name for the AST element type.
func TypeName(x any) string {
	return v1.TypeName(x)
}

// Location records a position in source code.
type Location = v1.Location

// NewLocation returns a new Location object.
func NewLocation(text []byte, file string, row int, col int) *Location {
	return v1.NewLocation(text, file, row, col)
}

// Value declares the common interface for all Term values. Every kind of Term value
// in the language is represented as a type that implements this interface:
//
// - Null, Boolean, Number, String
// - Object, Array, Set
// - Variables, References
// - Array, Set, and Object Comprehensions
// - Calls
type Value = v1.Value

// InterfaceToValue converts a native Go value x to a Value.
func InterfaceToValue(x any) (Value, error) {
	return v1.InterfaceToValue(x)
}

// ValueFromReader returns an AST value from a JSON serialized value in the reader.
func ValueFromReader(r io.Reader) (Value, error) {
	return v1.ValueFromReader(r)
}

// As converts v into a Go native type referred to by x.
func As(v Value, x any) error {
	return v1.As(v, x)
}

// Resolver defines the interface for resolving references to native Go values.
type Resolver = v1.Resolver

// ValueResolver defines the interface for resolving references to AST values.
type ValueResolver = v1.ValueResolver

// UnknownValueErr indicates a ValueResolver was unable to resolve a reference
// because the reference refers to an unknown value.
type UnknownValueErr = v1.UnknownValueErr

// IsUnknownValueErr returns true if the err is an UnknownValueErr.
func IsUnknownValueErr(err error) bool {
	return v1.IsUnknownValueErr(err)
}

// ValueToInterface returns the Go representation of an AST value.  The AST
// value should not contain any values that require evaluation (e.g., vars,
// comprehensions, etc.)
func ValueToInterface(v Value, resolver Resolver) (any, error) {
	return v1.ValueToInterface(v, resolver)
}

// JSON returns the JSON representation of v. The value must not contain any
// refs or terms that require evaluation (e.g., vars, comprehensions, etc.)
func JSON(v Value) (any, error) {
	return v1.JSON(v)
}

// JSONOpt defines parameters for AST to JSON conversion.
type JSONOpt = v1.JSONOpt

// JSONWithOpt returns the JSON representation of v. The value must not contain any
// refs or terms that require evaluation (e.g., vars, comprehensions, etc.)
func JSONWithOpt(v Value, opt JSONOpt) (any, error) {
	return v1.JSONWithOpt(v, opt)
}

// MustJSON returns the JSON representation of v. The value must not contain any
// refs or terms that require evaluation (e.g., vars, comprehensions, etc.) If
// the conversion fails, this function will panic. This function is mostly for
// test purposes.
func MustJSON(v Value) any {
	return v1.MustJSON(v)
}

// MustInterfaceToValue converts a native Go value x to a Value. If the
// conversion fails, this function will panic. This function is mostly for test
// purposes.
func MustInterfaceToValue(x any) Value {
	return v1.MustInterfaceToValue(x)
}

// Term is an argument to a function.
type Term = v1.Term

// NewTerm returns a new Term object.
func NewTerm(v Value) *Term {
	return v1.NewTerm(v)
}

// IsConstant returns true if the AST value is constant.
func IsConstant(v Value) bool {
	return v1.IsConstant(v)
}

// IsComprehension returns true if the supplied value is a comprehension.
func IsComprehension(x Value) bool {
	return v1.IsComprehension(x)
}

// ContainsRefs returns true if the Value v contains refs.
func ContainsRefs(v any) bool {
	return v1.ContainsRefs(v)
}

// ContainsComprehensions returns true if the Value v contains comprehensions.
func ContainsComprehensions(v any) bool {
	return v1.ContainsComprehensions(v)
}

// ContainsClosures returns true if the Value v contains closures.
func ContainsClosures(v any) bool {
	return v1.ContainsClosures(v)
}

// IsScalar returns true if the AST value is a scalar.
func IsScalar(v Value) bool {
	return v1.IsScalar(v)
}

// Null represents the null value defined by JSON.
type Null = v1.Null

// NullTerm creates a new Term with a Null value.
func NullTerm() *Term {
	return v1.NullTerm()
}

// Boolean represents a boolean value defined by JSON.
type Boolean = v1.Boolean

// BooleanTerm creates a new Term with a Boolean value.
func BooleanTerm(b bool) *Term {
	return v1.BooleanTerm(b)
}

// Number represents a numeric value as defined by JSON.
type Number = v1.Number

// NumberTerm creates a new Term with a Number value.
func NumberTerm(n json.Number) *Term {
	return v1.NumberTerm(n)
}

// IntNumberTerm creates a new Term with an integer Number value.
func IntNumberTerm(i int) *Term {
	return v1.IntNumberTerm(i)
}

// UIntNumberTerm creates a new Term with an unsigned integer Number value.
func UIntNumberTerm(u uint64) *Term {
	return v1.UIntNumberTerm(u)
}

// FloatNumberTerm creates a new Term with a floating point Number value.
func FloatNumberTerm(f float64) *Term {
	return v1.FloatNumberTerm(f)
}

// String represents a string value as defined by JSON.
type String = v1.String

// StringTerm creates a new Term with a String value.
func StringTerm(s string) *Term {
	return v1.StringTerm(s)
}

// Var represents a variable as defined by the language.
type Var = v1.Var

// VarTerm creates a new Term with a Variable value.
func VarTerm(v string) *Term {
	return v1.VarTerm(v)
}

// Ref represents a reference as defined by the language.
type Ref = v1.Ref

// EmptyRef returns a new, empty reference.
func EmptyRef() Ref {
	return v1.EmptyRef()
}

// PtrRef returns a new reference against the head for the pointer
// s. Path components in the pointer are unescaped.
func PtrRef(head *Term, s string) (Ref, error) {
	return v1.PtrRef(head, s)
}

// RefTerm creates a new Term with a Ref value.
func RefTerm(r ...*Term) *Term {
	return v1.RefTerm(r...)
}

func IsVarCompatibleString(s string) bool {
	return v1.IsVarCompatibleString(s)
}

// QueryIterator defines the interface for querying AST documents with references.
type QueryIterator = v1.QueryIterator

// ArrayTerm creates a new Term with an Array value.
func ArrayTerm(a ...*Term) *Term {
	return v1.ArrayTerm(a...)
}

// NewArray creates an Array with the terms provided. The array will
// use the provided term slice.
func NewArray(a ...*Term) *Array {
	return v1.NewArray(a...)
}

// Array represents an array as defined by the language. Arrays are similar to the
// same types as defined by JSON with the exception that they can contain Vars
// and References.
type Array = v1.Array

// Set represents a set as defined by the language.
type Set = v1.Set

// NewSet returns a new Set containing t.
func NewSet(t ...*Term) Set {
	return v1.NewSet(t...)
}

func SetTerm(t ...*Term) *Term {
	return v1.SetTerm(t...)
}

// Object represents an object as defined by the language.
type Object = v1.Object

// NewObject creates a new Object with t.
func NewObject(t ...[2]*Term) Object {
	return v1.NewObject(t...)
}

// ObjectTerm creates a new Term with an Object value.
func ObjectTerm(o ...[2]*Term) *Term {
	return v1.ObjectTerm(o...)
}

func LazyObject(blob map[string]any) Object {
	return v1.LazyObject(blob)
}

// Item is a helper for constructing an tuple containing two Terms
// representing a key/value pair in an Object.
func Item(key, value *Term) [2]*Term {
	return v1.Item(key, value)
}

// NOTE(philipc): The only way to get an ObjectKeyIterator should be
// from an Object. This ensures that the iterator can have implementation-
// specific details internally, with no contracts except to the very
// limited interface.
type ObjectKeysIterator = v1.ObjectKeysIterator

// ArrayComprehension represents an array comprehension as defined in the language.
type ArrayComprehension = v1.ArrayComprehension

// ArrayComprehensionTerm creates a new Term with an ArrayComprehension value.
func ArrayComprehensionTerm(term *Term, body Body) *Term {
	return v1.ArrayComprehensionTerm(term, body)
}

// ObjectComprehension represents an object comprehension as defined in the language.
type ObjectComprehension = v1.ObjectComprehension

// ObjectComprehensionTerm creates a new Term with an ObjectComprehension value.
func ObjectComprehensionTerm(key, value *Term, body Body) *Term {
	return v1.ObjectComprehensionTerm(key, value, body)
}

// SetComprehension represents a set comprehension as defined in the language.
type SetComprehension = v1.SetComprehension

// SetComprehensionTerm creates a new Term with an SetComprehension value.
func SetComprehensionTerm(term *Term, body Body) *Term {
	return v1.SetComprehensionTerm(term, body)
}

// Call represents as function call in the language.
type Call = v1.Call

// CallTerm returns a new Term with a Call value defined by terms. The first
// term is the operator and the rest are operands.
func CallTerm(terms ...*Term) *Term {
	return v1.CallTerm(terms...)
}

// Transformer defines the interface for transforming AST elements. If the
// transformer returns nil and does not indicate an error, the AST element will
// be set to nil and no transformations will be applied to children of the
// element.
type Transformer = v1.Transformer

// Transform iterates the AST and calls the Transform function on the
// Transformer t for x before recursing.
func Transform(t Transformer, x any) (any, error) {
	return v1.Transform(t, x)
}

// TransformRefs calls the function f on all references under x.
func TransformRefs(x any, f func(Ref) (Value, error)) (any, error) {
	return v1.TransformRefs(x, f)
}

// TransformVars calls the function f on all vars under x.
func TransformVars(x any, f func(Var) (Value, error)) (any, error) {
	return v1.TransformVars(x, f)
}

// TransformComprehensions calls the functio nf on all comprehensions under x.
func TransformComprehensions(x any, f func(any) (Value, error)) (any, error) {
	return v1.TransformComprehensions(x, f)
}

// GenericTransformer implements the Transformer interface to provide a utility
// to transform AST nodes using a closure.
type GenericTransformer = v1.GenericTransformer

// NewGenericTransformer returns a new GenericTransformer that will transform
// AST nodes using the function f.
func NewGenericTransformer(f func(x any) (any, error)) *GenericTransformer {
	return v1.NewGenericTransformer(f)
}

// Unify returns a set of variables that will be unified when the equality expression defined by
// terms a and b is evaluated. The unifier assumes that variables in the VarSet safe are already
// unified.
func Unify(safe VarSet, a *Term, b *Term) VarSet {
	return v1.Unify(safe, a, b)
}

// VarSet represents a set of variables.
type VarSet = v1.VarSet

// NewVarSet returns a new VarSet containing the specified variables.
func NewVarSet(vs ...Var) VarSet {
	return v1.NewVarSet(vs...)
}

// Visitor defines the interface for iterating AST elements. The Visit function
// can return a Visitor w which will be used to visit the children of the AST
// element v. If the Visit function returns nil, the children will not be
// visited.
//
// Deprecated: use GenericVisitor or another visitor implementation
type Visitor = v1.Visitor

// BeforeAndAfterVisitor wraps Visitor to provide hooks for being called before
// and after the AST has been visited.
//
// Deprecated: use GenericVisitor or another visitor implementation
type BeforeAndAfterVisitor = v1.BeforeAndAfterVisitor

// Walk iterates the AST by calling the Visit function on the Visitor
// v for x before recursing.
//
// Deprecated: use GenericVisitor.Walk
func Walk(v Visitor, x any) {
	v1.Walk(v, x)
}

// WalkBeforeAndAfter iterates the AST by calling the Visit function on the
// Visitor v for x before recursing.
//
// Deprecated: use GenericVisitor.Walk
func WalkBeforeAndAfter(v BeforeAndAfterVisitor, x any) {
	v1.WalkBeforeAndAfter(v, x)
}

// WalkVars calls the function f on all vars under x. If the function f
// returns true, AST nodes under the last node will not be visited.
func WalkVars(x any, f func(Var) bool) {
	v1.WalkVars(x, f)
}

// WalkClosures calls the function f on all closures under x. If the function f
// returns true, AST nodes under the last node will not be visited.
func WalkClosures(x any, f func(any) bool) {
	v1.WalkClosures(x, f)
}

// WalkRefs calls the function f on all references under x. If the function f
// returns true, AST nodes under the last node will not be visited.
func WalkRefs(x any, f func(Ref) bool) {
	v1.WalkRefs(x, f)
}

// WalkTerms calls the function f on all terms under x. If the function f
// returns true, AST nodes under the last node will not be visited.
func WalkTerms(x any, f func(*Term) bool) {
	v1.WalkTerms(x, f)
}

// WalkWiths calls the function f on all with modifiers under x. If the function f
// returns true, AST nodes under the last node will not be visited.
func WalkWiths(x any, f func(*With) bool) {
	v1.WalkWiths(x, f)
}

// WalkExprs calls the function f on all expressions under x. If the function f
// returns true, AST nodes under the last node will not be visited.
func WalkExprs(x any, f func(*Expr) bool) {
	v1.WalkExprs(x, f)
}

// WalkBodies calls the function f on all bodies under x. If the function f
// returns true, AST nodes under the last node will not be visited.
func WalkBodies(x any, f func(Body) bool) {
	v1.WalkBodies(x, f)
}

// WalkRules calls the function f on all rules under x. If the function f
// returns true, AST nodes under the last node will not be visited.
func WalkRules(x any, f func(*Rule) bool) {
	v1.WalkRules(x, f)
}

// WalkNodes calls the function f on all nodes under x. If the function f
// returns true, AST nodes under the last node will not be visited.
func WalkNodes(x any, f func(Node) bool) {
	v1.WalkNodes(x, f)
}

// GenericVisitor provides a utility to walk over AST nodes using a
// closure. If the closure returns true, the visitor will not walk
// over AST nodes under x.
type GenericVisitor = v1.GenericVisitor

// NewGenericVisitor returns a new GenericVisitor that will invoke the function
// f on AST nodes.
func NewGenericVisitor(f func(x any) bool) *GenericVisitor {
	return v1.NewGenericVisitor(f)
}

// BeforeAfterVisitor provides a utility to walk over AST nodes using
// closures. If the before closure returns true, the visitor will not
// walk over AST nodes under x. The after closure is invoked always
// after visiting a node.
type BeforeAfterVisitor = v1.BeforeAfterVisitor

// NewBeforeAfterVisitor returns a new BeforeAndAfterVisitor that
// will invoke the functions before and after AST nodes.
func NewBeforeAfterVisitor(before func(x any) bool, after func(x any)) *BeforeAfterVisitor {
	return v1.NewBeforeAfterVisitor(before, after)
}

// VarVisitor walks AST nodes under a given node and collects all encountered
// variables. The collected variables can be controlled by specifying
// VarVisitorParams when creating the visitor.
type VarVisitor = v1.VarVisitor

// VarVisitorParams contains settings for a VarVisitor.
type VarVisitorParams = v1.VarVisitorParams

// NewVarVisitor returns a new VarVisitor object.
func NewVarVisitor() *VarVisitor {
	return v1.NewVarVisitor()
}
