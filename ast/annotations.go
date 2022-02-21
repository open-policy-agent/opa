// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/open-policy-agent/opa/internal/deepcopy"
	"github.com/open-policy-agent/opa/util"
)

const (
	annotationScopePackage     = "package"
	annotationScopeImport      = "import"
	annotationScopeRule        = "rule"
	annotationScopeDocument    = "document"
	annotationScopeSubpackages = "subpackages"
)

type (
	// Annotations represents metadata attached to other AST nodes such as rules.
	Annotations struct {
		Location         *Location                    `json:"-"`
		Scope            string                       `json:"scope"`
		Title            string                       `json:"title,omitempty"`
		Description      string                       `json:"description,omitempty"`
		Organizations    []string                     `json:"organizations,omitempty"`
		RelatedResources []*RelatedResourceAnnotation `json:"related_resources,omitempty"`
		Authors          []*AuthorAnnotation          `json:"authors,omitempty"`
		Schemas          []*SchemaAnnotation          `json:"schemas,omitempty"`
		Custom           map[string]interface{}       `json:"custom,omitempty"`
		node             Node
	}

	// SchemaAnnotation contains a schema declaration for the document identified by the path.
	SchemaAnnotation struct {
		Path       Ref          `json:"path"`
		Schema     Ref          `json:"schema,omitempty"`
		Definition *interface{} `json:"definition,omitempty"`
	}

	AuthorAnnotation struct {
		Name  string `json:"name"`
		Email string `json:"email,omitempty"`
	}

	RelatedResourceAnnotation struct {
		Ref         url.URL `json:"ref"`
		Description string  `json:"description,omitempty"`
	}

	annotationSet struct {
		byRule    map[*Rule][]*Annotations
		byPackage map[*Package]*Annotations
		byPath    *annotationTreeNode
	}
)

func (a *Annotations) String() string {
	bs, _ := json.Marshal(a)
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

// Compare returns an integer indicating if s is less than, equal to, or greater
// than other.
func (a *Annotations) Compare(other *Annotations) int {

	if cmp := scopeCompare(a.Scope, other.Scope); cmp != 0 {
		return cmp
	}

	if cmp := strings.Compare(a.Title, other.Title); cmp != 0 {
		return cmp
	}

	if cmp := strings.Compare(a.Description, other.Description); cmp != 0 {
		return cmp
	}

	if cmp := compareStringLists(a.Organizations, other.Organizations); cmp != 0 {
		return cmp
	}

	if cmp := compareRelatedResources(a.RelatedResources, other.RelatedResources); cmp != 0 {
		return cmp
	}

	if cmp := compareAuthors(a.Authors, other.Authors); cmp != 0 {
		return cmp
	}

	if cmp := compareSchemas(a.Schemas, other.Schemas); cmp != 0 {
		return cmp
	}

	if cmp := util.Compare(a.Custom, other.Custom); cmp != 0 {
		return cmp
	}

	return 0
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
	switch s {
	case annotationScopeRule:
		return 1
	}
	return 0
}

func compareAuthors(a, b []*AuthorAnnotation) int {
	if len(a) > len(b) {
		return 1
	} else if len(a) < len(b) {
		return -1
	}

	for i := 0; i < len(a); i++ {
		if cmp := a[i].Compare(b[i]); cmp != 0 {
			return cmp
		}
	}

	return 0
}

func compareRelatedResources(a, b []*RelatedResourceAnnotation) int {
	if len(a) > len(b) {
		return 1
	} else if len(a) < len(b) {
		return -1
	}

	for i := 0; i < len(a); i++ {
		if cmp := strings.Compare(a[i].String(), b[i].String()); cmp != 0 {
			return cmp
		}
	}

	return 0
}

func compareSchemas(a, b []*SchemaAnnotation) int {
	max := len(a)
	if len(b) < max {
		max = len(b)
	}

	for i := 0; i < max; i++ {
		if cmp := a[i].Compare(b[i]); cmp != 0 {
			return cmp
		}
	}

	if len(a) > len(b) {
		return 1
	} else if len(a) < len(b) {
		return -1
	}

	return 0
}

func compareStringLists(a, b []string) int {
	if len(a) > len(b) {
		return 1
	} else if len(a) < len(b) {
		return -1
	}

	for i := 0; i < len(a); i++ {
		if cmp := strings.Compare(a[i], b[i]); cmp != 0 {
			return cmp
		}
	}

	return 0
}

// Copy returns a deep copy of s.
func (a *Annotations) Copy(node Node) *Annotations {
	cpy := *a

	cpy.Organizations = make([]string, len(a.Organizations))
	copy(cpy.Organizations, a.Organizations)

	cpy.RelatedResources = make([]*RelatedResourceAnnotation, len(a.RelatedResources))
	for i := range a.RelatedResources {
		cpy.RelatedResources[i] = a.RelatedResources[i].Copy()
	}

	cpy.Authors = make([]*AuthorAnnotation, len(a.Authors))
	for i := range a.Authors {
		cpy.Authors[i] = a.Authors[i].Copy()
	}

	cpy.Schemas = make([]*SchemaAnnotation, len(a.Schemas))
	for i := range a.Schemas {
		cpy.Schemas[i] = a.Schemas[i].Copy()
	}

	cpy.Custom = deepcopy.Map(a.Custom)

	cpy.node = node

	return &cpy
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

	if cmp := strings.Compare(a.Email, other.Email); cmp != 0 {
		return cmp
	}

	return 0
}

func (a *AuthorAnnotation) String() string {
	bs, _ := json.Marshal(a)
	return string(bs)
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

	if cmp := strings.Compare(rr.Ref.String(), other.Ref.String()); cmp != 0 {
		return cmp
	}

	return 0
}

func (rr *RelatedResourceAnnotation) String() string {
	bs, _ := json.Marshal(rr)
	return string(bs)
}

func (rr *RelatedResourceAnnotation) MarshalJSON() ([]byte, error) {
	d := map[string]interface{}{
		"ref": rr.Ref.String(),
	}

	if len(rr.Description) > 0 {
		d["description"] = rr.Description
	}

	return json.Marshal(d)
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

	if s.Definition != nil && other.Definition == nil {
		return -1
	} else if s.Definition == nil && other.Definition != nil {
		return 1
	} else if s.Definition != nil && other.Definition != nil {
		return util.Compare(*s.Definition, *other.Definition)
	}

	return 0
}

func (s *SchemaAnnotation) String() string {
	bs, _ := json.Marshal(s)
	return string(bs)
}

func newAnnotationSet() *annotationSet {
	return &annotationSet{
		byRule:    map[*Rule][]*Annotations{},
		byPackage: map[*Package]*Annotations{},
		byPath:    newAnnotationTree(),
	}
}

func buildAnnotationSet(rules []util.T) (*annotationSet, Errors) {
	as := newAnnotationSet()
	processed := map[*Module]struct{}{}
	var errs Errors
	for _, x := range rules {
		module := x.(*Rule).Module
		if _, ok := processed[module]; ok {
			continue
		}
		processed[module] = struct{}{}
		for _, a := range module.Annotations {
			if err := as.add(a); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return as, nil
}

func (as *annotationSet) add(a *Annotations) *Error {
	switch a.Scope {
	case annotationScopeRule:
		rule := a.node.(*Rule)
		as.byRule[rule] = append(as.byRule[rule], a)
	case annotationScopePackage:
		pkg := a.node.(*Package)
		if exist, ok := as.byPackage[pkg]; ok {
			return errAnnotationRedeclared(a, exist.Location)
		}
		as.byPackage[pkg] = a
	case annotationScopeDocument:
		rule := a.node.(*Rule)
		path := rule.Path()
		x := as.byPath.get(path)
		if x != nil {
			return errAnnotationRedeclared(a, x.Value.Location)
		}
		as.byPath.insert(path, a)
	case annotationScopeSubpackages:
		pkg := a.node.(*Package)
		x := as.byPath.get(pkg.Path)
		if x != nil && x.Value != nil {
			return errAnnotationRedeclared(a, x.Value.Location)
		}
		as.byPath.insert(pkg.Path, a)
	}
	return nil
}

func (as *annotationSet) getRuleScope(r *Rule) []*Annotations {
	if as == nil {
		return nil
	}
	return as.byRule[r]
}

func (as *annotationSet) getSubpackagesScope(path Ref) []*Annotations {
	if as == nil {
		return nil
	}
	return as.byPath.ancestors(path)
}

func (as *annotationSet) getDocumentScope(path Ref) *Annotations {
	if as == nil {
		return nil
	}
	if node := as.byPath.get(path); node != nil {
		return node.Value
	}
	return nil
}

func (as *annotationSet) getPackageScope(pkg *Package) *Annotations {
	if as == nil {
		return nil
	}
	return as.byPackage[pkg]
}

type annotationTreeNode struct {
	Value    *Annotations
	Children map[Value]*annotationTreeNode // we assume key elements are hashable (vars and strings only!)
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

// mergeAnnotations merges a slice of annotations into one.
// The passed annotations slice must be ordered less significant to more significant; e.g. [package x, package x.y, rule x.y.z].
func mergeAnnotationsList(annotations []*Annotations) *Annotations {
	if len(annotations) == 0 {
		return nil
	}

	var result *Annotations

	if len(annotations) == 1 {
		result = annotations[0].Copy(nil)
	} else {
		result = annotations[0]

		for _, b := range annotations[1:] {
			result = mergeAnnotations(result, b)
		}
	}

	// It makes little sense to keep any of these annotations,
	// as they're too specific to their respective set of annotations.
	result.Location = nil
	result.node = nil
	result.Scope = ""

	return result
}

// merge returns a new Annotations with any annotation present in the given other replaced.
func mergeAnnotations(a *Annotations, b *Annotations) *Annotations {
	result := a.Copy(nil)
	bCopy := b.Copy(nil)

	if len(bCopy.Title) > 0 {
		result.Title = bCopy.Title
	}

	if len(bCopy.Description) > 0 {
		result.Description = bCopy.Description
	}

	if len(bCopy.Organizations) > 0 {
		result.Organizations = bCopy.Organizations
	}

	if len(bCopy.RelatedResources) > 0 {
		result.RelatedResources = bCopy.RelatedResources
	}

	if len(bCopy.Authors) > 0 {
		result.Authors = bCopy.Authors
	}

	if len(bCopy.Schemas) > 0 {
		result.Schemas = bCopy.Schemas
	}

	if len(bCopy.Custom) > 0 {
		result.Custom = bCopy.Custom
	}

	return result
}
