// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	"github.com/open-policy-agent/opa/internal/deepcopy"
	"github.com/open-policy-agent/opa/v1/util"
)

const (
	annotationScopePackage     = "package"
	annotationScopeRule        = "rule"
	annotationScopeDocument    = "document"
	annotationScopeSubpackages = "subpackages"
)

type (
	// Annotations represents metadata attached to other AST nodes such as rules.
	Annotations struct {
		Scope            string                       `json:"scope"`
		Title            string                       `json:"title,omitempty"`
		Entrypoint       bool                         `json:"entrypoint,omitempty"`
		Description      string                       `json:"description,omitempty"`
		Organizations    []string                     `json:"organizations,omitempty"`
		RelatedResources []*RelatedResourceAnnotation `json:"related_resources,omitempty"`
		Authors          []*AuthorAnnotation          `json:"authors,omitempty"`
		Schemas          []*SchemaAnnotation          `json:"schemas,omitempty"`
		Compile          *CompileAnnotation           `json:"compile,omitempty"`
		Custom           map[string]any               `json:"custom,omitempty"`
		Labels           map[string]any               `json:"labels,omitempty"`
		Location         *Location                    `json:"location,omitempty"`

		endLoc *Location
		node   Node
	}

	// SchemaAnnotation contains a schema declaration for the document identified by the path.
	SchemaAnnotation struct {
		Path       Ref  `json:"path"`
		Schema     Ref  `json:"schema,omitempty"`
		Definition *any `json:"definition,omitempty"`
	}

	CompileAnnotation struct {
		Unknowns []Ref `json:"unknowns,omitempty"`
		MaskRule Ref   `json:"mask_rule,omitempty"` // NOTE: This doesn't need to start with "data.package", it can be relative
	}

	AuthorAnnotation struct {
		Name  string `json:"name"`
		Email string `json:"email,omitempty"`
	}

	RelatedResourceAnnotation struct {
		Ref         url.URL `json:"ref"`
		Description string  `json:"description,omitempty"`
	}

	AnnotationSet struct {
		byRule    map[*Rule][]*Annotations
		byPackage map[int]*Annotations
		byPath    *annotationTreeNode
		modules   []*Module // Modules this set was constructed from
	}

	annotationTreeNode struct {
		Value    *Annotations
		Children map[Value]*annotationTreeNode // we assume key elements are hashable (vars and strings only!)
	}

	AnnotationsRef struct {
		Path        Ref          `json:"path"` // The path of the node the annotations are applied to
		Annotations *Annotations `json:"annotations,omitempty"`
		Location    *Location    `json:"location,omitempty"` // The location of the node the annotations are applied to

		node Node // The node the annotations are applied to
	}

	AnnotationsRefSet []*AnnotationsRef

	FlatAnnotationsRefSet AnnotationsRefSet
)

func (a *Annotations) String() string {
	bs, _ := a.MarshalJSON()
	return string(bs)
}

// Loc returns the location of this annotation.
func (a *Annotations) Loc() *Location {
	return a.Location
}

// SetLoc updates the location of this annotation.
func (a *Annotations) SetLoc(l *Location) {
	a.Location = l
}

// EndLoc returns the location of this annotation's last comment line.
func (a *Annotations) EndLoc() *Location {
	return util.NilOr(a.endLoc, a.Location)
}

// Compare returns an integer indicating if a is less than, equal to, or greater
// than other.
func (a *Annotations) Compare(other *Annotations) int {
	if a == other {
		return 0
	}
	if a == nil {
		return -1
	}
	if other == nil {
		return 1
	}

	if cmp := scopeCompare(a.Scope, other.Scope); cmp != 0 {
		return cmp
	}

	if cmp := strings.Compare(a.Title, other.Title); cmp != 0 {
		return cmp
	}

	if cmp := strings.Compare(a.Description, other.Description); cmp != 0 {
		return cmp
	}

	if cmp := slices.Compare(a.Organizations, other.Organizations); cmp != 0 {
		return cmp
	}

	if cmp := slices.CompareFunc(a.RelatedResources, other.RelatedResources, (*RelatedResourceAnnotation).Compare); cmp != 0 {
		return cmp
	}

	if cmp := slices.CompareFunc(a.Authors, other.Authors, (*AuthorAnnotation).Compare); cmp != 0 {
		return cmp
	}

	if cmp := slices.CompareFunc(a.Schemas, other.Schemas, (*SchemaAnnotation).Compare); cmp != 0 {
		return cmp
	}

	if cmp := a.Compile.Compare(other.Compile); cmp != 0 {
		return cmp
	}

	if a.Entrypoint != other.Entrypoint {
		if a.Entrypoint {
			return 1
		}
		return -1
	}

	if cmp := util.Compare(a.Custom, other.Custom); cmp != 0 {
		return cmp
	}

	return util.Compare(a.Labels, other.Labels)
}

// GetTargetPath returns the path of the node these Annotations are applied to (the target)
func (a *Annotations) GetTargetPath() Ref {
	switch n := a.node.(type) {
	case *Package:
		return n.Path
	case *Rule:
		return n.Ref().GroundPrefix()
	default:
		return nil
	}
}

func NewAnnotationsRef(a *Annotations) *AnnotationsRef {
	var loc *Location
	if a.node != nil {
		loc = a.node.Loc()
	}

	return &AnnotationsRef{
		Location:    loc,
		Path:        a.GetTargetPath(),
		Annotations: a,
		node:        a.node,
	}
}

func (ar *AnnotationsRef) GetPackage() *Package {
	switch n := ar.node.(type) {
	case *Package:
		return n
	case *Rule:
		return n.Module.Package
	default:
		return nil
	}
}

func (ar *AnnotationsRef) GetRule() *Rule {
	if r, ok := ar.node.(*Rule); ok {
		return r
	}
	return nil
}

func scopeCompare(s1, s2 string) int {
	o1 := scopeOrder(s1)
	o2 := scopeOrder(s2)

	if o2 < o1 {
		return 1
	} else if o2 > o1 {
		return -1
	}

	if s1 < s2 {
		return -1
	} else if s2 < s1 {
		return 1
	}

	return 0
}

func scopeOrder(s string) int {
	if s == annotationScopeRule {
		return 1
	}
	return 0
}

// Copy returns a deep copy of s.
func (a *Annotations) Copy(node Node) *Annotations {
	cpy := *a
	cpy.Organizations = slices.Clone(a.Organizations)
	cpy.RelatedResources = util.Map(a.RelatedResources, (*RelatedResourceAnnotation).Copy)
	cpy.Authors = util.Map(a.Authors, (*AuthorAnnotation).Copy)
	cpy.Schemas = util.Map(a.Schemas, (*SchemaAnnotation).Copy)
	cpy.Compile = a.Compile.Copy()

	if a.Custom != nil {
		cpy.Custom = deepcopy.Map(a.Custom)
	}
	if a.Labels != nil {
		cpy.Labels = deepcopy.Map(a.Labels)
	}
	cpy.node = node

	return &cpy
}

// toTerm constructs an AST Object from the annotation, and wraps it in a *Term.
func (a *Annotations) toTerm() (*Term, *Error) {
	if a == nil {
		return ObjectTerm(), nil
	}

	items := make([][2]*Term, 0, util.Count(util.Identity,
		a.Entrypoint,
		len(a.Scope) > 0,
		len(a.Title) > 0,
		len(a.Description) > 0,
		len(a.Organizations) > 0,
		len(a.RelatedResources) > 0,
		len(a.Authors) > 0,
		len(a.Schemas) > 0,
		len(a.Custom) > 0,
		len(a.Labels) > 0,
	))

	if len(a.Scope) > 0 {
		items = append(items, [2]*Term{InternedTerm("scope"), InternedTerm(a.Scope)})
	}

	if len(a.Title) > 0 {
		items = append(items, [2]*Term{InternedTerm("title"), StringTerm(a.Title)})
	}

	if a.Entrypoint {
		items = append(items, [2]*Term{InternedTerm("entrypoint"), InternedTerm(true)})
	}

	if len(a.Description) > 0 {
		items = append(items, [2]*Term{InternedTerm("description"), StringTerm(a.Description)})
	}

	if len(a.Organizations) > 0 {
		items = append(items, [2]*Term{InternedTerm("organizations"), ArrayTerm(util.Map(a.Organizations, StringTerm)...)})
	}

	if len(a.RelatedResources) > 0 {
		rrs := util.Map(a.RelatedResources, (*RelatedResourceAnnotation).toTerm)
		items = append(items, [2]*Term{InternedTerm("related_resources"), ArrayTerm(rrs...)})
	}

	if len(a.Authors) > 0 {
		as := util.Map(a.Authors, (*AuthorAnnotation).toTerm)
		items = append(items, [2]*Term{InternedTerm("authors"), ArrayTerm(as...)})
	}

	if len(a.Schemas) > 0 {
		ss, err := util.TryMap(a.Schemas, (*SchemaAnnotation).toTerm)
		if err != nil {
			return nil, NewError(CompileErr, a.Location, "invalid schema annotation %s", err.Error())
		}
		items = append(items, [2]*Term{InternedTerm("schemas"), ArrayTerm(ss...)})
	}

	if len(a.Custom) > 0 {
		c, err := InterfaceToValue(a.Custom)
		if err != nil {
			return nil, NewError(CompileErr, a.Location, "invalid custom annotation %s", err.Error())
		}
		items = append(items, [2]*Term{InternedTerm("custom"), NewTerm(c)})
	}

	if len(a.Labels) > 0 {
		l, err := InterfaceToValue(a.Labels)
		if err != nil {
			return nil, NewError(CompileErr, a.Location, "invalid labels annotation %s", err.Error())
		}
		items = append(items, [2]*Term{InternedTerm("labels"), NewTerm(l)})
	}

	return ObjectTerm(items...), nil
}

func attachRuleAnnotations(mod *Module) {
	// make a copy of the annotations
	cpy := make([]*Annotations, len(mod.Annotations))
	for i, a := range mod.Annotations {
		cpy[i] = a.Copy(a.node)
	}

	for _, rule := range mod.Rules {
		var j int
		var found bool
		for i, a := range cpy {
			if rule.Ref().GroundPrefix().Equal(a.GetTargetPath()) {
				if a.Scope == annotationScopeDocument {
					rule.Annotations = append(rule.Annotations, a)
				} else if a.Scope == annotationScopeRule && rule.Loc().Row > a.Location.Row {
					j = i
					found = true
					rule.Annotations = append(rule.Annotations, a)
				}
			}
		}

		if found && j < len(cpy) {
			cpy = slices.Delete(cpy, j, j+1)
		}
	}
}

func attachAnnotationsNodes(mod *Module) Errors {
	var errs Errors
	// Find first non-annotation statement following each annotation and attach
	// the annotation to that statement.
	for _, a := range mod.Annotations {
		for _, stmt := range mod.stmts {
			if _, ok := stmt.(*Annotations); !ok {
				if stmt.Loc().Row > a.Location.Row {
					a.node = stmt
					break
				}
			}
		}

		if a.Scope == "" {
			switch a.node.(type) {
			case *Rule:
				a.Scope = annotationScopeRule
				if a.Entrypoint {
					a.Scope = annotationScopeDocument
				}
			case *Package:
				a.Scope = annotationScopePackage
			case *Import:
				// Note that this isn't a valid scope, but set here so that the
				// validate function called below can print an error message with
				// a context that makes sense ("invalid scope: 'import'" instead of
				// "invalid scope: '')
				a.Scope = "import"
			}
		}

		if err := validateAnnotationScopeAttachment(a); err != nil {
			errs = append(errs, err)
		}

		if err := validateAnnotationEntrypointAttachment(a); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func validateAnnotationScopeAttachment(a *Annotations) *Error {
	switch a.Scope {
	case annotationScopeRule, annotationScopeDocument:
		if _, ok := a.node.(*Rule); ok {
			return nil
		}
		return newScopeAttachmentErr(a, "rule")
	case annotationScopePackage, annotationScopeSubpackages:
		if _, ok := a.node.(*Package); ok {
			return nil
		}
		return newScopeAttachmentErr(a, "package")
	}

	return NewError(ParseErr, a.Loc(), "invalid annotation scope '%v'. Use one of '%s', '%s', '%s', or '%s'",
		a.Scope, annotationScopeRule, annotationScopeDocument, annotationScopePackage, annotationScopeSubpackages)
}

func validateAnnotationEntrypointAttachment(a *Annotations) *Error {
	if a.Entrypoint && !(a.Scope == annotationScopeDocument || a.Scope == annotationScopePackage) {
		return NewError(
			ParseErr, a.Loc(), "annotation entrypoint applied to non-document or package scope '%v'", a.Scope)
	}
	return nil
}

// Copy returns a deep copy of a.
func (a *AuthorAnnotation) Copy() *AuthorAnnotation {
	cpy := *a
	return &cpy
}

// Compare returns an integer indicating if s is less than, equal to, or greater
// than other.
func (a *AuthorAnnotation) Compare(other *AuthorAnnotation) int {
	if cmp := strings.Compare(a.Name, other.Name); cmp != 0 {
		return cmp
	}
	return strings.Compare(a.Email, other.Email)
}

func (a *AuthorAnnotation) String() string {
	if len(a.Email) == 0 {
		return a.Name
	} else if len(a.Name) == 0 {
		return fmt.Sprintf("<%s>", a.Email)
	}
	return fmt.Sprintf("%s <%s>", a.Name, a.Email)
}

func (a *AuthorAnnotation) toTerm() *Term {
	items := make([][2]*Term, 0, 2)
	if len(a.Name) > 0 {
		items = append(items, [2]*Term{InternedTerm("name"), StringTerm(a.Name)})
	}
	if len(a.Email) > 0 {
		items = append(items, [2]*Term{InternedTerm("email"), StringTerm(a.Email)})
	}
	return ObjectTerm(items...)
}

// Copy returns a deep copy of rr.
func (rr *RelatedResourceAnnotation) Copy() *RelatedResourceAnnotation {
	cpy := *rr
	return &cpy
}

// Compare returns an integer indicating if s is less than, equal to, or greater
// than other.
func (rr *RelatedResourceAnnotation) Compare(other *RelatedResourceAnnotation) int {
	if cmp := strings.Compare(rr.Description, other.Description); cmp != 0 {
		return cmp
	}
	return strings.Compare(rr.Ref.String(), other.Ref.String())
}

func (rr *RelatedResourceAnnotation) String() string {
	bs, _ := json.Marshal(rr)
	return string(bs)
}

func (rr *RelatedResourceAnnotation) toTerm() *Term {
	items := make([][2]*Term, 0, 2)
	if len(rr.Ref.String()) > 0 {
		items = append(items, [2]*Term{InternedTerm("ref"), StringTerm(rr.Ref.String())})
	}
	if len(rr.Description) > 0 {
		items = append(items, [2]*Term{InternedTerm("description"), StringTerm(rr.Description)})
	}
	return ObjectTerm(items...)
}

// Copy returns a deep copy of s.
func (s *SchemaAnnotation) Copy() *SchemaAnnotation {
	cpy := *s
	return &cpy
}

// Compare returns an integer indicating if s is less than, equal to, or greater
// than other.
func (s *SchemaAnnotation) Compare(other *SchemaAnnotation) int {
	if cmp := s.Path.Compare(other.Path); cmp != 0 {
		return cmp
	}
	if cmp := s.Schema.Compare(other.Schema); cmp != 0 {
		return cmp
	}

	switch {
	case s.Definition == other.Definition:
		return 0
	case s.Definition == nil:
		return -1
	case other.Definition == nil:
		return 1
	}

	return util.Compare(*s.Definition, *other.Definition)
}

func (s *SchemaAnnotation) String() string {
	bs, _ := json.Marshal(s)
	return string(bs)
}

func (s *SchemaAnnotation) toTerm() (*Term, error) {
	items := make([][2]*Term, 0, 3)
	if len(s.Path.String()) > 0 {
		items = append(items, [2]*Term{InternedTerm("path"), NewTerm(s.Path.toArray())})
	}
	if len(s.Schema.String()) > 0 {
		items = append(items, [2]*Term{InternedTerm("schema"), NewTerm(s.Schema.toArray())})
	}
	if s.Definition != nil {
		def, err := InterfaceToValue(s.Definition)
		if err != nil {
			return nil, err
		}
		items = append(items, [2]*Term{InternedTerm("definition"), NewTerm(def)})
	}
	return ObjectTerm(items...), nil
}

// Copy returns a deep copy of s.
func (c *CompileAnnotation) Copy() *CompileAnnotation {
	if c == nil {
		return nil
	}
	cpy := *c
	cpy.Unknowns = util.Map(c.Unknowns, Ref.Copy)
	return &cpy
}

// Compare returns an integer indicating if s is less than, equal to, or greater
// than other.
func (c *CompileAnnotation) Compare(other *CompileAnnotation) int {
	switch {
	case c == other:
		return 0
	case c == nil:
		return -1
	case other == nil:
		return 1
	}

	if cmp := slices.CompareFunc(c.Unknowns, other.Unknowns, RefCompare); cmp != 0 {
		return cmp
	}
	return c.MaskRule.Compare(other.MaskRule)
}

func (c *CompileAnnotation) String() string {
	bs, _ := json.Marshal(c)
	return string(bs)
}

func newAnnotationSet() *AnnotationSet {
	return &AnnotationSet{
		byRule:    map[*Rule][]*Annotations{},
		byPackage: map[int]*Annotations{},
		byPath:    newAnnotationTree(),
	}
}

func BuildAnnotationSet(modules []*Module) (*AnnotationSet, Errors) {
	as := newAnnotationSet()
	var errs Errors
	for _, m := range modules {
		for _, a := range m.Annotations {
			if err := as.add(a); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	as.modules = modules
	return as, nil
}

// NOTE(philipc): During copy propagation, the underlying Nodes can be
// stripped away from the annotations, leading to nil deref panics. We
// silently ignore these cases for now, as a workaround.
func (as *AnnotationSet) add(a *Annotations) *Error {
	switch a.Scope {
	case annotationScopeRule:
		if rule, ok := a.node.(*Rule); ok {
			as.byRule[rule] = append(as.byRule[rule], a)
		}
	case annotationScopePackage:
		if pkg, ok := a.node.(*Package); ok {
			hash := pkg.Path.Hash()
			if exist, ok := as.byPackage[hash]; ok {
				return errAnnotationRedeclared(a, exist.Location)
			}
			as.byPackage[hash] = a
		}
	case annotationScopeDocument:
		if rule, ok := a.node.(*Rule); ok {
			path := rule.Ref().GroundPrefix()
			x := as.byPath.get(path)
			if x != nil {
				return errAnnotationRedeclared(a, x.Value.Location)
			}
			as.byPath.insert(path, a)
		}
	case annotationScopeSubpackages:
		if pkg, ok := a.node.(*Package); ok {
			x := as.byPath.get(pkg.Path)
			if x != nil && x.Value != nil {
				return errAnnotationRedeclared(a, x.Value.Location)
			}
			as.byPath.insert(pkg.Path, a)
		}
	}
	return nil
}

func (as *AnnotationSet) GetRuleScope(r *Rule) []*Annotations {
	if as == nil {
		return nil
	}
	return as.byRule[r]
}

func (as *AnnotationSet) GetSubpackagesScope(path Ref) []*Annotations {
	if as == nil {
		return nil
	}
	return as.byPath.ancestors(path)
}

func (as *AnnotationSet) GetDocumentScope(path Ref) *Annotations {
	if as == nil {
		return nil
	}
	if node := as.byPath.get(path); node != nil {
		return node.Value
	}
	return nil
}

func (as *AnnotationSet) GetPackageScope(pkg *Package) *Annotations {
	if as == nil {
		return nil
	}
	return as.byPackage[pkg.Path.Hash()]
}

// Flatten returns a flattened list view of this AnnotationSet.
// The returned slice is sorted, first by the annotations' target path, then by their target location
func (as *AnnotationSet) Flatten() FlatAnnotationsRefSet {
	// This preallocation often won't be optimal, but it's superior to starting with a nil slice.
	size := len(as.byPath.Children) + len(as.byRule) + len(as.byPackage)
	refs := as.byPath.flatten(make([]*AnnotationsRef, 0, size))
	for _, a := range as.byPackage {
		refs = append(refs, NewAnnotationsRef(a))
	}

	for _, as := range as.byRule {
		refs = util.MapAppend(refs, as, NewAnnotationsRef)
	}

	// Sort by path, then annotation location, for stable output
	return util.SortedStableFunc(refs, (*AnnotationsRef).Compare)
}

// Chain returns the chain of annotations leading up to the given rule.
// The returned slice is ordered as follows
// 0. Entries for the given rule, ordered from the METADATA block declared immediately above the rule, to the block declared farthest away (always at least one entry)
// 1. The 'document' scope entry, if any
// 2. The 'package' scope entry, if any
// 3. Entries for the 'subpackages' scope, if any; ordered from the closest package path to the fartest. E.g.: 'do.re.mi', 'do.re', 'do'
// The returned slice is guaranteed to always contain at least one entry, corresponding to the given rule.
func (as *AnnotationSet) Chain(rule *Rule) AnnotationsRefSet {
	ruleAnnots := as.GetRuleScope(rule)
	// Fall back to the rule's own attached annotations when the rule's source
	// module isn't tracked by this AnnotationSet. This happens for rules
	// supplied by an ExternalRuleSource that returns []*Rule directly: their
	// source module is never compiled by the outer Compiler, so the set has no
	// entries for them at any scope. attachRuleAnnotations (run at parse time)
	// populates rule.Annotations with rule-scope and document-scope entries,
	// which is the upper bound of what's reachable for such rules anyway.
	if len(ruleAnnots) == 0 && len(rule.Annotations) > 0 && !slices.Contains(as.modules, rule.Module) {
		ruleAnnots = rule.Annotations
	}

	var refs []*AnnotationsRef
	if len(ruleAnnots) >= 1 {
		// Sort by annotation location; chain must start with annotations declared closest to rule, then going outward
		refs = util.SortedStableFunc(util.Map(ruleAnnots, NewAnnotationsRef), func(a, b *AnnotationsRef) int {
			return -a.Annotations.Location.Compare(b.Annotations.Location)
		})
	} else {
		// Make sure there is always a leading entry representing the passed rule, even if it has no annotations
		refs = append(refs, &AnnotationsRef{
			Location: rule.Location,
			Path:     rule.Ref().GroundPrefix(),
			node:     rule,
		})
	}

	if da := as.GetDocumentScope(rule.Ref().GroundPrefix()); da != nil {
		refs = append(refs, NewAnnotationsRef(da))
	}

	if pa := as.GetPackageScope(rule.Module.Package); pa != nil {
		refs = append(refs, NewAnnotationsRef(pa))
	}

	subPkgAnnots := as.GetSubpackagesScope(rule.Module.Package.Path)
	// We need to reverse the order, as subPkgAnnots ordering will start at the root,
	// whereas we want to end at the root.
	for _, subPkgAnnot := range slices.Backward(subPkgAnnots) {
		refs = append(refs, NewAnnotationsRef(subPkgAnnot))
	}

	return refs
}

// MergedLabels returns the inner-scope-wins merged labels for the given rule
// along with a stable JSON string suitable for content-based deduplication.
// labels is nil when the rule has no labels anywhere in its annotation chain.
func (as *AnnotationSet) MergedLabels(rule *Rule) (labels map[string]any, key string) {
	if as == nil {
		return nil, ""
	}
	labels = mergeChainLabels(as.Chain(rule))
	if len(labels) > 0 {
		b, _ := json.Marshal(labels)
		key = string(b)
	}
	return labels, key
}

// mergeChainLabels folds labels from a rule's annotation chain with inner-wins
// precedence. AnnotationSet.Chain returns entries in inner-to-outer order, so
// we iterate in reverse to fold outer-to-inner.
func mergeChainLabels(chain AnnotationsRefSet) map[string]any {
	var merged map[string]any
	for _, c := range slices.Backward(chain) {
		a := c.Annotations
		if a == nil || len(a.Labels) == 0 {
			continue
		}
		if merged == nil {
			merged = make(map[string]any, len(a.Labels))
		}
		maps.Copy(merged, a.Labels)
	}
	return merged
}

func (ars FlatAnnotationsRefSet) Insert(ar *AnnotationsRef) FlatAnnotationsRefSet {
	result := make(FlatAnnotationsRefSet, 0, len(ars)+1)

	// insertion sort, first by path, then location
	for i, current := range ars {
		if ar.Compare(current) < 0 {
			result = append(append(result, ar), ars[i:]...)
			break
		}
		result = append(result, current)
	}

	if len(result) < len(ars)+1 {
		result = append(result, ar)
	}

	return result
}

func newAnnotationTree() *annotationTreeNode {
	return &annotationTreeNode{
		Value:    nil,
		Children: map[Value]*annotationTreeNode{},
	}
}

func (t *annotationTreeNode) insert(path Ref, value *Annotations) {
	node := t
	for _, k := range path {
		child, ok := node.Children[k.Value]
		if !ok {
			child = newAnnotationTree()
			node.Children[k.Value] = child
		}
		node = child
	}
	node.Value = value
}

func (t *annotationTreeNode) get(path Ref) *annotationTreeNode {
	node := t
	for _, k := range path {
		if node == nil {
			return nil
		}
		child, ok := node.Children[k.Value]
		if !ok {
			return nil
		}
		node = child
	}
	return node
}

// ancestors returns a slice of annotations in ascending order, starting with the root of ref; e.g.: 'root', 'root.foo', 'root.foo.bar'.
func (t *annotationTreeNode) ancestors(path Ref) (result []*Annotations) {
	node := t
	for _, k := range path {
		if node == nil {
			return result
		}
		child, ok := node.Children[k.Value]
		if !ok {
			return result
		}
		if child.Value != nil {
			result = append(result, child.Value)
		}
		node = child
	}
	return result
}

func (t *annotationTreeNode) flatten(refs []*AnnotationsRef) []*AnnotationsRef {
	if a := t.Value; a != nil {
		refs = append(refs, NewAnnotationsRef(a))
	}
	for _, c := range t.Children {
		refs = c.flatten(refs)
	}
	return refs
}

func (ar *AnnotationsRef) Compare(other *AnnotationsRef) int {
	if c := ar.Path.Compare(other.Path); c != 0 {
		return c
	}
	if c := ar.Annotations.Location.Compare(other.Annotations.Location); c != 0 {
		return c
	}
	return ar.Annotations.Compare(other.Annotations)
}
