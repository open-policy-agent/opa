// Copyright 2024 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"reflect"

	"github.com/huandu/go-clone"
)

const (
	cteMarkerInit injectionMarker = iota
	cteMarkerAfterWith
)

// With creates a new CTE builder with default flavor.
func With(tables ...*CTEQueryBuilder) *CTEBuilder {
	return DefaultFlavor.NewCTEBuilder().With(tables...)
}

// WithRecursive creates a new recursive CTE builder with default flavor.
func WithRecursive(tables ...*CTEQueryBuilder) *CTEBuilder {
	return DefaultFlavor.NewCTEBuilder().WithRecursive(tables...)
}

func newCTEBuilder() *CTEBuilder {
	return &CTEBuilder{
		args:      &Args{},
		injection: newInjection(),
	}
}

// Clone returns a deep copy of CTEBuilder.
// It's useful when you want to create a base builder and clone it to build similar queries.
func (cteb *CTEBuilder) Clone() *CTEBuilder {
	return clone.Clone(cteb).(*CTEBuilder)
}

func init() {
	t := reflect.TypeOf(CTEBuilder{})
	clone.SetCustomFunc(t, func(allocator *clone.Allocator, old, new reflect.Value) {
		cloned := allocator.CloneSlowly(old)
		new.Set(cloned)

		cteb := cloned.Addr().Interface().(*CTEBuilder)
		for i, b := range cteb.queries {
			cteb.args.Replace(cteb.queryBuilderVars[i], b)
		}
	})
}

// CTEBuilder is a CTE (Common Table Expression) builder.
type CTEBuilder struct {
	recursive        bool
	queries          []*CTEQueryBuilder
	queryBuilderVars []string

	args *Args

	injection *injection
	marker    injectionMarker
}

var _ Builder = new(CTEBuilder)

// With sets the CTE name and columns.
func (cteb *CTEBuilder) With(queries ...*CTEQueryBuilder) *CTEBuilder {
	queryBuilderVars := make([]string, 0, len(queries))

	for _, query := range queries {
		queryBuilderVars = append(queryBuilderVars, cteb.args.Add(query))
	}

	cteb.queries = append([]*CTEQueryBuilder(nil), queries...)
	cteb.queryBuilderVars = queryBuilderVars
	cteb.marker = cteMarkerAfterWith
	return cteb
}

// WithRecursive sets the CTE name and columns and turns on the RECURSIVE keyword.
func (cteb *CTEBuilder) WithRecursive(queries ...*CTEQueryBuilder) *CTEBuilder {
	cteb.With(queries...).recursive = true
	return cteb
}

// Select creates a new SelectBuilder to build a SELECT statement using this CTE.
func (cteb *CTEBuilder) Select(col ...string) *SelectBuilder {
	sb := cteb.args.Flavor.NewSelectBuilder()
	return sb.With(cteb).Select(col...)
}

// DeleteFrom creates a new DeleteBuilder to build a DELETE statement using this CTE.
func (cteb *CTEBuilder) DeleteFrom(table string) *DeleteBuilder {
	db := cteb.args.Flavor.NewDeleteBuilder()
	return db.With(cteb).DeleteFrom(table)
}

// Update creates a new UpdateBuilder to build an UPDATE statement using this CTE.
func (cteb *CTEBuilder) Update(table string) *UpdateBuilder {
	ub := cteb.args.Flavor.NewUpdateBuilder()
	return ub.With(cteb).Update(table)
}

// String returns the compiled CTE string.
func (cteb *CTEBuilder) String() string {
	sql, _ := cteb.Build()
	return sql
}

// Build returns compiled CTE string and args.
func (cteb *CTEBuilder) Build() (sql string, args []interface{}) {
	return cteb.BuildWithFlavor(cteb.args.Flavor)
}

// BuildWithFlavor builds a CTE with the specified flavor and initial arguments.
func (cteb *CTEBuilder) BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	buf := newStringBuilder()
	cteb.injection.WriteTo(buf, cteMarkerInit)

	if len(cteb.queryBuilderVars) > 0 {
		buf.WriteLeadingString("WITH ")
		if cteb.recursive {
			buf.WriteString("RECURSIVE ")
		}
		buf.WriteStrings(cteb.queryBuilderVars, ", ")
	}

	cteb.injection.WriteTo(buf, cteMarkerAfterWith)
	return cteb.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
}

// SetFlavor sets the flavor of compiled sql.
func (cteb *CTEBuilder) SetFlavor(flavor Flavor) (old Flavor) {
	old = cteb.args.Flavor
	cteb.args.Flavor = flavor
	return
}

// Flavor returns flavor of builder
func (cteb *CTEBuilder) Flavor() Flavor {
	return cteb.args.Flavor
}

// SQL adds an arbitrary sql to current position.
func (cteb *CTEBuilder) SQL(sql string) *CTEBuilder {
	cteb.injection.SQL(cteb.marker, sql)
	return cteb
}

// TableNames returns all table names in a CTE.
func (cteb *CTEBuilder) TableNames() []string {
	if len(cteb.queryBuilderVars) == 0 {
		return nil
	}

	tableNames := make([]string, 0, len(cteb.queries))

	for _, query := range cteb.queries {
		tableNames = append(tableNames, query.TableName())
	}

	return tableNames
}

// tableNamesForFrom returns a list of table names which should be automatically added to FROM clause.
// It's not public, as this feature is designed only for SelectBuilder/UpdateBuilder/DeleteBuilder right now.
func (cteb *CTEBuilder) tableNamesForFrom() []string {
	cnt := 0

	// ShouldAddToTableList() unlikely returns true.
	// Count it before allocating any memory for better performance.
	for _, query := range cteb.queries {
		if query.ShouldAddToTableList() {
			cnt++
		}
	}

	if cnt == 0 {
		return nil
	}

	tableNames := make([]string, 0, cnt)

	for _, query := range cteb.queries {
		if query.ShouldAddToTableList() {
			tableNames = append(tableNames, query.TableName())
		}
	}

	return tableNames
}
