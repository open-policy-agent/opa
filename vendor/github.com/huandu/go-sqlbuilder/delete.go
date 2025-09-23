// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"reflect"

	"github.com/huandu/go-clone"
)

const (
	deleteMarkerInit injectionMarker = iota
	deleteMarkerAfterWith
	deleteMarkerAfterDeleteFrom
	deleteMarkerAfterWhere
	deleteMarkerAfterOrderBy
	deleteMarkerAfterLimit
	deleteMarkerAfterReturning
)

// NewDeleteBuilder creates a new DELETE builder.
func NewDeleteBuilder() *DeleteBuilder {
	return DefaultFlavor.NewDeleteBuilder()
}

func newDeleteBuilder() *DeleteBuilder {
	args := &Args{}
	proxy := &whereClauseProxy{}
	return &DeleteBuilder{
		whereClauseProxy: proxy,
		whereClauseExpr:  args.Add(proxy),

		Cond: Cond{
			Args: args,
		},
		args:      args,
		injection: newInjection(),
	}
}

// Clone returns a deep copy of DeleteBuilder.
// It's useful when you want to create a base builder and clone it to build similar queries.
func (db *DeleteBuilder) Clone() *DeleteBuilder {
	return clone.Clone(db).(*DeleteBuilder)
}

func init() {
	t := reflect.TypeOf(DeleteBuilder{})
	clone.SetCustomFunc(t, func(allocator *clone.Allocator, old, new reflect.Value) {
		cloned := allocator.CloneSlowly(old)
		new.Set(cloned)

		db := cloned.Addr().Interface().(*DeleteBuilder)
		db.args.Replace(db.whereClauseExpr, db.whereClauseProxy)
		db.args.Replace(db.cteBuilderVar, db.cteBuilder)
	})
}

// DeleteBuilder is a builder to build DELETE.
type DeleteBuilder struct {
	*WhereClause
	Cond

	whereClauseProxy *whereClauseProxy
	whereClauseExpr  string

	cteBuilderVar string
	cteBuilder    *CTEBuilder

	tables      []string
	orderByCols []string
	order       string
	limitVar    string
	returning   []string

	args *Args

	injection *injection
	marker    injectionMarker
}

var _ Builder = new(DeleteBuilder)

// DeleteFrom sets table name in DELETE.
func DeleteFrom(table ...string) *DeleteBuilder {
	return DefaultFlavor.NewDeleteBuilder().DeleteFrom(table...)
}

// With sets WITH clause (the Common Table Expression) before DELETE.
func (db *DeleteBuilder) With(builder *CTEBuilder) *DeleteBuilder {
	db.marker = deleteMarkerAfterWith
	db.cteBuilderVar = db.Var(builder)
	db.cteBuilder = builder
	return db
}

// DeleteFrom sets table name in DELETE.
func (db *DeleteBuilder) DeleteFrom(table ...string) *DeleteBuilder {
	db.tables = table
	db.marker = deleteMarkerAfterDeleteFrom
	return db
}

// TableNames returns all table names in this DELETE statement.
func (db *DeleteBuilder) TableNames() []string {
	var additionalTableNames []string

	if db.cteBuilder != nil {
		additionalTableNames = db.cteBuilder.tableNamesForFrom()
	}

	var tableNames []string

	if len(db.tables) > 0 && len(additionalTableNames) > 0 {
		tableNames = make([]string, len(db.tables)+len(additionalTableNames))
		copy(tableNames, db.tables)
		copy(tableNames[len(db.tables):], additionalTableNames)
	} else if len(db.tables) > 0 {
		tableNames = db.tables
	} else if len(additionalTableNames) > 0 {
		tableNames = additionalTableNames
	}

	return tableNames
}

// Where sets expressions of WHERE in DELETE.
func (db *DeleteBuilder) Where(andExpr ...string) *DeleteBuilder {
	if len(andExpr) == 0 || estimateStringsBytes(andExpr) == 0 {
		return db
	}

	if db.WhereClause == nil {
		db.WhereClause = NewWhereClause()
	}

	db.WhereClause.AddWhereExpr(db.args, andExpr...)
	db.marker = deleteMarkerAfterWhere
	return db
}

// AddWhereClause adds all clauses in the whereClause to SELECT.
func (db *DeleteBuilder) AddWhereClause(whereClause *WhereClause) *DeleteBuilder {
	if db.WhereClause == nil {
		db.WhereClause = NewWhereClause()
	}

	db.WhereClause.AddWhereClause(whereClause)
	return db
}

// OrderBy sets columns of ORDER BY in DELETE.
func (db *DeleteBuilder) OrderBy(col ...string) *DeleteBuilder {
	db.orderByCols = col
	db.marker = deleteMarkerAfterOrderBy
	return db
}

// Asc sets order of ORDER BY to ASC.
func (db *DeleteBuilder) Asc() *DeleteBuilder {
	db.order = "ASC"
	db.marker = deleteMarkerAfterOrderBy
	return db
}

// Desc sets order of ORDER BY to DESC.
func (db *DeleteBuilder) Desc() *DeleteBuilder {
	db.order = "DESC"
	db.marker = deleteMarkerAfterOrderBy
	return db
}

// Limit sets the LIMIT in DELETE.
func (db *DeleteBuilder) Limit(limit int) *DeleteBuilder {
	if limit < 0 {
		db.limitVar = ""
		return db
	}

	db.limitVar = db.Var(limit)
	db.marker = deleteMarkerAfterLimit
	return db
}

// Returning sets returning columns.
// For DBMS that doesn't support RETURNING, e.g. MySQL, it will be ignored.
func (db *DeleteBuilder) Returning(col ...string) *DeleteBuilder {
	db.returning = col
	db.marker = deleteMarkerAfterReturning
	return db
}

// String returns the compiled DELETE string.
func (db *DeleteBuilder) String() string {
	s, _ := db.Build()
	return s
}

// Build returns compiled DELETE string and args.
// They can be used in `DB#Query` of package `database/sql` directly.
func (db *DeleteBuilder) Build() (sql string, args []interface{}) {
	return db.BuildWithFlavor(db.args.Flavor)
}

// BuildWithFlavor returns compiled DELETE string and args with flavor and initial args.
// They can be used in `DB#Query` of package `database/sql` directly.
func (db *DeleteBuilder) BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	buf := newStringBuilder()
	db.injection.WriteTo(buf, deleteMarkerInit)

	if db.cteBuilder != nil {
		buf.WriteLeadingString(db.cteBuilderVar)
		db.injection.WriteTo(buf, deleteMarkerAfterWith)
	}

	tableNames := db.TableNames()

	if len(tableNames) > 0 {
		buf.WriteLeadingString("DELETE FROM ")
		buf.WriteStrings(tableNames, ", ")
	}

	db.injection.WriteTo(buf, deleteMarkerAfterDeleteFrom)

	if db.WhereClause != nil {
		db.whereClauseProxy.WhereClause = db.WhereClause
		defer func() {
			db.whereClauseProxy.WhereClause = nil
		}()

		buf.WriteLeadingString(db.whereClauseExpr)
		db.injection.WriteTo(buf, deleteMarkerAfterWhere)
	}

	if len(db.orderByCols) > 0 {
		buf.WriteLeadingString("ORDER BY ")
		buf.WriteStrings(db.orderByCols, ", ")

		if db.order != "" {
			buf.WriteRune(' ')
			buf.WriteString(db.order)
		}

		db.injection.WriteTo(buf, deleteMarkerAfterOrderBy)
	}

	if len(db.limitVar) > 0 {
		buf.WriteLeadingString("LIMIT ")
		buf.WriteString(db.limitVar)

		db.injection.WriteTo(buf, deleteMarkerAfterLimit)
	}

	if flavor == PostgreSQL || flavor == SQLite {
		if len(db.returning) > 0 {
			buf.WriteLeadingString("RETURNING ")
			buf.WriteStrings(db.returning, ", ")
		}

		db.injection.WriteTo(buf, deleteMarkerAfterReturning)
	}

	return db.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
}

// SetFlavor sets the flavor of compiled sql.
func (db *DeleteBuilder) SetFlavor(flavor Flavor) (old Flavor) {
	old = db.args.Flavor
	db.args.Flavor = flavor
	return
}

// Flavor returns flavor of builder
func (db *DeleteBuilder) Flavor() Flavor {
	return db.args.Flavor
}

// SQL adds an arbitrary sql to current position.
func (db *DeleteBuilder) SQL(sql string) *DeleteBuilder {
	db.injection.SQL(db.marker, sql)
	return db
}
