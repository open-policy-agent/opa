// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"reflect"

	"github.com/huandu/go-clone"
)

const (
	updateMarkerInit injectionMarker = iota
	updateMarkerAfterWith
	updateMarkerAfterUpdate
	updateMarkerAfterSet
	updateMarkerAfterWhere
	updateMarkerAfterOrderBy
	updateMarkerAfterLimit
	updateMarkerAfterReturning
)

// NewUpdateBuilder creates a new UPDATE builder.
func NewUpdateBuilder() *UpdateBuilder {
	return DefaultFlavor.NewUpdateBuilder()
}

func newUpdateBuilder() *UpdateBuilder {
	args := &Args{}
	proxy := &whereClauseProxy{}
	return &UpdateBuilder{
		whereClauseProxy: proxy,
		whereClauseExpr:  args.Add(proxy),

		Cond: Cond{
			Args: args,
		},
		args:      args,
		injection: newInjection(),
	}
}

// Clone returns a deep copy of UpdateBuilder.
// It's useful when you want to create a base builder and clone it to build similar queries.
func (ub *UpdateBuilder) Clone() *UpdateBuilder {
	return clone.Clone(ub).(*UpdateBuilder)
}

func init() {
	t := reflect.TypeOf(UpdateBuilder{})
	clone.SetCustomFunc(t, func(allocator *clone.Allocator, old, new reflect.Value) {
		cloned := allocator.CloneSlowly(old)
		new.Set(cloned)

		ub := cloned.Addr().Interface().(*UpdateBuilder)
		ub.args.Replace(ub.whereClauseExpr, ub.whereClauseProxy)
		ub.args.Replace(ub.cteBuilderVar, ub.cteBuilder)
	})
}

// UpdateBuilder is a builder to build UPDATE.
type UpdateBuilder struct {
	*WhereClause
	Cond

	whereClauseProxy *whereClauseProxy
	whereClauseExpr  string

	cteBuilderVar string
	cteBuilder    *CTEBuilder

	tables      []string
	assignments []string
	orderByCols []string
	order       string
	limitVar    string
	returning   []string

	args *Args

	injection *injection
	marker    injectionMarker
}

var _ Builder = new(UpdateBuilder)

// Update sets table name in UPDATE.
func Update(table ...string) *UpdateBuilder {
	return DefaultFlavor.NewUpdateBuilder().Update(table...)
}

// With sets WITH clause (the Common Table Expression) before UPDATE.
func (ub *UpdateBuilder) With(builder *CTEBuilder) *UpdateBuilder {
	ub.marker = updateMarkerAfterWith
	ub.cteBuilderVar = ub.Var(builder)
	ub.cteBuilder = builder
	return ub
}

// Update sets table name in UPDATE.
func (ub *UpdateBuilder) Update(table ...string) *UpdateBuilder {
	ub.tables = table
	ub.marker = updateMarkerAfterUpdate
	return ub
}

// TableNames returns all table names in this UPDATE statement.
func (ub *UpdateBuilder) TableNames() (tableNames []string) {
	var additionalTableNames []string

	if ub.cteBuilder != nil {
		additionalTableNames = ub.cteBuilder.tableNamesForFrom()
	}

	if len(ub.tables) > 0 && len(additionalTableNames) > 0 {
		tableNames = make([]string, len(ub.tables)+len(additionalTableNames))
		copy(tableNames, ub.tables)
		copy(tableNames[len(ub.tables):], additionalTableNames)
	} else if len(ub.tables) > 0 {
		tableNames = ub.tables
	} else if len(additionalTableNames) > 0 {
		tableNames = additionalTableNames
	}

	return tableNames
}

// Set sets the assignments in SET.
func (ub *UpdateBuilder) Set(assignment ...string) *UpdateBuilder {
	ub.assignments = assignment
	ub.marker = updateMarkerAfterSet
	return ub
}

// SetMore appends the assignments in SET.
func (ub *UpdateBuilder) SetMore(assignment ...string) *UpdateBuilder {
	ub.assignments = append(ub.assignments, assignment...)
	ub.marker = updateMarkerAfterSet
	return ub
}

// Where sets expressions of WHERE in UPDATE.
func (ub *UpdateBuilder) Where(andExpr ...string) *UpdateBuilder {
	if len(andExpr) == 0 || estimateStringsBytes(andExpr) == 0 {
		return ub
	}

	if ub.WhereClause == nil {
		ub.WhereClause = NewWhereClause()
	}

	ub.WhereClause.AddWhereExpr(ub.args, andExpr...)
	ub.marker = updateMarkerAfterWhere
	return ub
}

// AddWhereClause adds all clauses in the whereClause to SELECT.
func (ub *UpdateBuilder) AddWhereClause(whereClause *WhereClause) *UpdateBuilder {
	if ub.WhereClause == nil {
		ub.WhereClause = NewWhereClause()
	}

	ub.WhereClause.AddWhereClause(whereClause)
	return ub
}

// Assign represents SET "field = value" in UPDATE.
func (ub *UpdateBuilder) Assign(field string, value interface{}) string {
	return fmt.Sprintf("%s = %s", Escape(field), ub.args.Add(value))
}

// Incr represents SET "field = field + 1" in UPDATE.
func (ub *UpdateBuilder) Incr(field string) string {
	f := Escape(field)
	return fmt.Sprintf("%s = %s + 1", f, f)
}

// Decr represents SET "field = field - 1" in UPDATE.
func (ub *UpdateBuilder) Decr(field string) string {
	f := Escape(field)
	return fmt.Sprintf("%s = %s - 1", f, f)
}

// Add represents SET "field = field + value" in UPDATE.
func (ub *UpdateBuilder) Add(field string, value interface{}) string {
	f := Escape(field)
	return fmt.Sprintf("%s = %s + %s", f, f, ub.args.Add(value))
}

// Sub represents SET "field = field - value" in UPDATE.
func (ub *UpdateBuilder) Sub(field string, value interface{}) string {
	f := Escape(field)
	return fmt.Sprintf("%s = %s - %s", f, f, ub.args.Add(value))
}

// Mul represents SET "field = field * value" in UPDATE.
func (ub *UpdateBuilder) Mul(field string, value interface{}) string {
	f := Escape(field)
	return fmt.Sprintf("%s = %s * %s", f, f, ub.args.Add(value))
}

// Div represents SET "field = field / value" in UPDATE.
func (ub *UpdateBuilder) Div(field string, value interface{}) string {
	f := Escape(field)
	return fmt.Sprintf("%s = %s / %s", f, f, ub.args.Add(value))
}

// OrderBy sets columns of ORDER BY in UPDATE.
func (ub *UpdateBuilder) OrderBy(col ...string) *UpdateBuilder {
	ub.orderByCols = col
	ub.marker = updateMarkerAfterOrderBy
	return ub
}

// Asc sets order of ORDER BY to ASC.
func (ub *UpdateBuilder) Asc() *UpdateBuilder {
	ub.order = "ASC"
	ub.marker = updateMarkerAfterOrderBy
	return ub
}

// Desc sets order of ORDER BY to DESC.
func (ub *UpdateBuilder) Desc() *UpdateBuilder {
	ub.order = "DESC"
	ub.marker = updateMarkerAfterOrderBy
	return ub
}

// Limit sets the LIMIT in UPDATE.
func (ub *UpdateBuilder) Limit(limit int) *UpdateBuilder {
	if limit < 0 {
		ub.limitVar = ""
		return ub
	}

	ub.limitVar = ub.Var(limit)
	ub.marker = updateMarkerAfterLimit
	return ub
}

// Returning sets returning columns.
// For DBMS that doesn't support RETURNING, e.g. MySQL, it will be ignored.
func (ub *UpdateBuilder) Returning(col ...string) *UpdateBuilder {
	ub.returning = col
	ub.marker = updateMarkerAfterReturning
	return ub
}

// NumAssignment returns the number of assignments to update.
func (ub *UpdateBuilder) NumAssignment() int {
	return len(ub.assignments)
}

// String returns the compiled UPDATE string.
func (ub *UpdateBuilder) String() string {
	s, _ := ub.Build()
	return s
}

// Build returns compiled UPDATE string and args.
// They can be used in `DB#Query` of package `database/sql` directly.
func (ub *UpdateBuilder) Build() (sql string, args []interface{}) {
	return ub.BuildWithFlavor(ub.args.Flavor)
}

// BuildWithFlavor returns compiled UPDATE string and args with flavor and initial args.
// They can be used in `DB#Query` of package `database/sql` directly.
func (ub *UpdateBuilder) BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	buf := newStringBuilder()
	ub.injection.WriteTo(buf, updateMarkerInit)

	if ub.cteBuilder != nil {
		buf.WriteLeadingString(ub.cteBuilderVar)
		ub.injection.WriteTo(buf, updateMarkerAfterWith)
	}

	switch flavor {
	case MySQL:
		// CTE table names should be written after UPDATE keyword in MySQL.
		tableNames := ub.TableNames()

		if len(tableNames) > 0 {
			buf.WriteLeadingString("UPDATE ")
			buf.WriteStrings(tableNames, ", ")
		}

	default:
		if len(ub.tables) > 0 {
			buf.WriteLeadingString("UPDATE ")
			buf.WriteStrings(ub.tables, ", ")
		}
	}

	ub.injection.WriteTo(buf, updateMarkerAfterUpdate)

	if assignments := filterEmptyStrings(ub.assignments); len(assignments) > 0 {
		buf.WriteLeadingString("SET ")
		buf.WriteStrings(assignments, ", ")
	}

	ub.injection.WriteTo(buf, updateMarkerAfterSet)

	if flavor != MySQL {
		// For ISO SQL, CTE table names should be written after FROM keyword.
		if ub.cteBuilder != nil {
			cteTableNames := ub.cteBuilder.tableNamesForFrom()

			if len(cteTableNames) > 0 {
				buf.WriteLeadingString("FROM ")
				buf.WriteStrings(cteTableNames, ", ")
			}
		}
	}

	if ub.WhereClause != nil {
		ub.whereClauseProxy.WhereClause = ub.WhereClause
		defer func() {
			ub.whereClauseProxy.WhereClause = nil
		}()

		buf.WriteLeadingString(ub.whereClauseExpr)
		ub.injection.WriteTo(buf, updateMarkerAfterWhere)
	}

	if len(ub.orderByCols) > 0 {
		buf.WriteLeadingString("ORDER BY ")
		buf.WriteStrings(ub.orderByCols, ", ")

		if ub.order != "" {
			buf.WriteLeadingString(ub.order)
		}

		ub.injection.WriteTo(buf, updateMarkerAfterOrderBy)
	}

	if len(ub.limitVar) > 0 {
		buf.WriteLeadingString("LIMIT ")
		buf.WriteString(ub.limitVar)

		ub.injection.WriteTo(buf, updateMarkerAfterLimit)
	}

	if flavor == PostgreSQL || flavor == SQLite {
		if len(ub.returning) > 0 {
			buf.WriteLeadingString("RETURNING ")
			buf.WriteStrings(ub.returning, ", ")
		}

		ub.injection.WriteTo(buf, updateMarkerAfterReturning)
	}

	return ub.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
}

// SetFlavor sets the flavor of compiled sql.
func (ub *UpdateBuilder) SetFlavor(flavor Flavor) (old Flavor) {
	old = ub.args.Flavor
	ub.args.Flavor = flavor
	return
}

// Flavor returns flavor of builder
func (ub *UpdateBuilder) Flavor() Flavor {
	return ub.args.Flavor
}

// SQL adds an arbitrary sql to current position.
func (ub *UpdateBuilder) SQL(sql string) *UpdateBuilder {
	ub.injection.SQL(ub.marker, sql)
	return ub
}
