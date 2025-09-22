// Copyright 2024 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"reflect"

	"github.com/huandu/go-clone"
)

const (
	cteQueryMarkerInit injectionMarker = iota
	cteQueryMarkerAfterTable
	cteQueryMarkerAfterAs
)

// CTETable creates a new CTE query builder with default flavor, marking it as a table.
//
// The resulting CTE query can be used in a `SelectBuilder“, where its table name will be
// automatically included in the FROM clause.
func CTETable(name string, cols ...string) *CTEQueryBuilder {
	return DefaultFlavor.NewCTEQueryBuilder().AddToTableList().Table(name, cols...)
}

// CTEQuery creates a new CTE query builder with default flavor.
func CTEQuery(name string, cols ...string) *CTEQueryBuilder {
	return DefaultFlavor.NewCTEQueryBuilder().Table(name, cols...)
}

func newCTEQueryBuilder() *CTEQueryBuilder {
	return &CTEQueryBuilder{
		args:      &Args{},
		injection: newInjection(),
	}
}

// Clone returns a deep copy of CTEQueryBuilder.
// It's useful when you want to create a base builder and clone it to build similar queries.
func (ctetb *CTEQueryBuilder) Clone() *CTEQueryBuilder {
	return clone.Clone(ctetb).(*CTEQueryBuilder)
}

func init() {
	t := reflect.TypeOf(CTEQueryBuilder{})
	clone.SetCustomFunc(t, func(allocator *clone.Allocator, old, new reflect.Value) {
		cloned := allocator.CloneSlowly(old)
		new.Set(cloned)

		ctetb := cloned.Addr().Interface().(*CTEQueryBuilder)
		ctetb.args.Replace(ctetb.builderVar, ctetb.builder)
	})
}

// CTEQueryBuilder is a builder to build one table in CTE (Common Table Expression).
type CTEQueryBuilder struct {
	name       string
	cols       []string
	builder    Builder
	builderVar string

	// if true, this query's table name will be automatically added to the table list
	// in FROM clause of SELECT statement.
	autoAddToTableList bool

	args *Args

	injection *injection
	marker    injectionMarker
}

var _ Builder = new(CTEQueryBuilder)

// CTETableBuilder is an alias of CTEQueryBuilder for backward compatibility.
//
// Deprecated: use CTEQueryBuilder instead.
type CTETableBuilder = CTEQueryBuilder

// Table sets the table name and columns in a CTE table.
func (ctetb *CTEQueryBuilder) Table(name string, cols ...string) *CTEQueryBuilder {
	ctetb.name = name
	ctetb.cols = cols
	ctetb.marker = cteQueryMarkerAfterTable
	return ctetb
}

// As sets the builder to select data.
func (ctetb *CTEQueryBuilder) As(builder Builder) *CTEQueryBuilder {
	ctetb.builder = builder
	ctetb.builderVar = ctetb.args.Add(builder)
	ctetb.marker = cteQueryMarkerAfterAs
	return ctetb
}

// AddToTableList sets flag to add table name to table list in FROM clause of SELECT statement.
func (ctetb *CTEQueryBuilder) AddToTableList() *CTEQueryBuilder {
	ctetb.autoAddToTableList = true
	return ctetb
}

// ShouldAddToTableList returns flag to add table name to table list in FROM clause of SELECT statement.
func (ctetb *CTEQueryBuilder) ShouldAddToTableList() bool {
	return ctetb.autoAddToTableList
}

// String returns the compiled CTE string.
func (ctetb *CTEQueryBuilder) String() string {
	sql, _ := ctetb.Build()
	return sql
}

// Build returns compiled CTE string and args.
func (ctetb *CTEQueryBuilder) Build() (sql string, args []interface{}) {
	return ctetb.BuildWithFlavor(ctetb.args.Flavor)
}

// BuildWithFlavor builds a CTE with the specified flavor and initial arguments.
func (ctetb *CTEQueryBuilder) BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	buf := newStringBuilder()
	ctetb.injection.WriteTo(buf, cteQueryMarkerInit)

	if ctetb.name != "" {
		buf.WriteLeadingString(ctetb.name)

		if len(ctetb.cols) > 0 {
			buf.WriteLeadingString("(")
			buf.WriteStrings(ctetb.cols, ", ")
			buf.WriteString(")")
		}

		ctetb.injection.WriteTo(buf, cteQueryMarkerAfterTable)
	}

	if ctetb.builderVar != "" {
		buf.WriteLeadingString("AS (")
		buf.WriteString(ctetb.builderVar)
		buf.WriteRune(')')

		ctetb.injection.WriteTo(buf, cteQueryMarkerAfterAs)
	}

	return ctetb.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
}

// SetFlavor sets the flavor of compiled sql.
func (ctetb *CTEQueryBuilder) SetFlavor(flavor Flavor) (old Flavor) {
	old = ctetb.args.Flavor
	ctetb.args.Flavor = flavor
	return
}

// Flavor returns flavor of builder
func (ctetb *CTEQueryBuilder) Flavor() Flavor {
	return ctetb.args.Flavor
}

// SQL adds an arbitrary sql to current position.
func (ctetb *CTEQueryBuilder) SQL(sql string) *CTEQueryBuilder {
	ctetb.injection.SQL(ctetb.marker, sql)
	return ctetb
}

// TableName returns the CTE table name.
func (ctetb *CTEQueryBuilder) TableName() string {
	return ctetb.name
}
