// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"strings"

	"github.com/huandu/go-clone"
)

const (
	insertMarkerInit injectionMarker = iota
	insertMarkerAfterInsertInto
	insertMarkerAfterCols
	insertMarkerAfterValues
	insertMarkerAfterSelect
	insertMarkerAfterReturning
)

// NewInsertBuilder creates a new INSERT builder.
func NewInsertBuilder() *InsertBuilder {
	return DefaultFlavor.NewInsertBuilder()
}

func newInsertBuilder() *InsertBuilder {
	args := &Args{}
	return &InsertBuilder{
		verb:      "INSERT",
		args:      args,
		injection: newInjection(),
	}
}

// Clone returns a deep copy of InsertBuilder.
// It's useful when you want to create a base builder and clone it to build similar queries.
func (ib *InsertBuilder) Clone() *InsertBuilder {
	return clone.Clone(ib).(*InsertBuilder)
}

// InsertBuilder is a builder to build INSERT.
type InsertBuilder struct {
	verb      string
	table     string
	cols      []string
	values    [][]string
	returning []string

	args *Args

	injection *injection
	marker    injectionMarker

	sbHolder string
}

var _ Builder = new(InsertBuilder)

// InsertInto sets table name in INSERT.
func InsertInto(table string) *InsertBuilder {
	return DefaultFlavor.NewInsertBuilder().InsertInto(table)
}

// InsertInto sets table name in INSERT.
func (ib *InsertBuilder) InsertInto(table string) *InsertBuilder {
	ib.table = Escape(table)
	ib.marker = insertMarkerAfterInsertInto
	return ib
}

// InsertIgnoreInto sets table name in INSERT IGNORE.
func InsertIgnoreInto(table string) *InsertBuilder {
	return DefaultFlavor.NewInsertBuilder().InsertIgnoreInto(table)
}

// InsertIgnoreInto sets table name in INSERT IGNORE.
func (ib *InsertBuilder) InsertIgnoreInto(table string) *InsertBuilder {
	ib.args.Flavor.PrepareInsertIgnore(table, ib)
	return ib
}

// ReplaceInto sets table name and changes the verb of ib to REPLACE.
// REPLACE INTO is a MySQL extension to the SQL standard.
func ReplaceInto(table string) *InsertBuilder {
	return DefaultFlavor.NewInsertBuilder().ReplaceInto(table)
}

// ReplaceInto sets table name and changes the verb of ib to REPLACE.
// REPLACE INTO is a MySQL extension to the SQL standard.
func (ib *InsertBuilder) ReplaceInto(table string) *InsertBuilder {
	ib.verb = "REPLACE"
	ib.table = Escape(table)
	ib.marker = insertMarkerAfterInsertInto
	return ib
}

// Cols sets columns in INSERT.
func (ib *InsertBuilder) Cols(col ...string) *InsertBuilder {
	ib.cols = EscapeAll(col...)
	ib.marker = insertMarkerAfterCols
	return ib
}

// Select returns a new SelectBuilder to build a SELECT statement inside the INSERT INTO.
func (isb *InsertBuilder) Select(col ...string) *SelectBuilder {
	sb := Select(col...)
	isb.sbHolder = isb.args.Add(sb)
	return sb
}

// Values adds a list of values for a row in INSERT.
func (ib *InsertBuilder) Values(value ...interface{}) *InsertBuilder {
	placeholders := make([]string, 0, len(value))

	for _, v := range value {
		placeholders = append(placeholders, ib.args.Add(v))
	}

	ib.values = append(ib.values, placeholders)
	ib.marker = insertMarkerAfterValues
	return ib
}

// Returning sets returning columns.
// For DBMS that doesn't support RETURNING, e.g. MySQL, it will be ignored.
func (ib *InsertBuilder) Returning(col ...string) *InsertBuilder {
	ib.returning = col
	ib.marker = insertMarkerAfterReturning
	return ib
}

// NumValue returns the number of values to insert.
func (ib *InsertBuilder) NumValue() int {
	return len(ib.values)
}

// String returns the compiled INSERT string.
func (ib *InsertBuilder) String() string {
	s, _ := ib.Build()
	return s
}

// Build returns compiled INSERT string and args.
// They can be used in `DB#Query` of package `database/sql` directly.
func (ib *InsertBuilder) Build() (sql string, args []interface{}) {
	return ib.BuildWithFlavor(ib.args.Flavor)
}

// BuildWithFlavor returns compiled INSERT string and args with flavor and initial args.
// They can be used in `DB#Query` of package `database/sql` directly.
func (ib *InsertBuilder) BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	buf := newStringBuilder()
	ib.injection.WriteTo(buf, insertMarkerInit)

	if len(ib.values) > 1 && ib.args.Flavor == Oracle {
		buf.WriteLeadingString(ib.verb)
		buf.WriteString(" ALL")

		for _, v := range ib.values {
			if len(ib.table) > 0 {
				buf.WriteString(" INTO ")
				buf.WriteString(ib.table)
			}
			ib.injection.WriteTo(buf, insertMarkerAfterInsertInto)
			if len(ib.cols) > 0 {
				buf.WriteLeadingString("(")
				buf.WriteStrings(ib.cols, ", ")
				buf.WriteString(")")

				ib.injection.WriteTo(buf, insertMarkerAfterCols)
			}

			buf.WriteLeadingString("VALUES ")
			values := make([]string, 0, len(ib.values))
			values = append(values, fmt.Sprintf("(%v)", strings.Join(v, ", ")))
			buf.WriteStrings(values, ", ")
		}

		buf.WriteString(" SELECT 1 from DUAL")

		ib.injection.WriteTo(buf, insertMarkerAfterValues)

		return ib.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
	}

	if len(ib.table) > 0 {
		buf.WriteLeadingString(ib.verb)
		buf.WriteString(" INTO ")
		buf.WriteString(ib.table)
	}

	ib.injection.WriteTo(buf, insertMarkerAfterInsertInto)

	if len(ib.cols) > 0 {
		buf.WriteLeadingString("(")
		buf.WriteStrings(ib.cols, ", ")
		buf.WriteString(")")

		ib.injection.WriteTo(buf, insertMarkerAfterCols)
	}

	if ib.sbHolder != "" {
		buf.WriteString(" ")
		buf.WriteString(ib.sbHolder)

		ib.injection.WriteTo(buf, insertMarkerAfterSelect)
	} else if len(ib.values) > 0 {
		buf.WriteLeadingString("VALUES ")
		values := make([]string, 0, len(ib.values))

		for _, v := range ib.values {
			values = append(values, fmt.Sprintf("(%v)", strings.Join(v, ", ")))
		}

		buf.WriteStrings(values, ", ")
	}

	ib.injection.WriteTo(buf, insertMarkerAfterValues)

	if flavor == PostgreSQL || flavor == SQLite {
		if len(ib.returning) > 0 {
			buf.WriteLeadingString("RETURNING ")
			buf.WriteStrings(ib.returning, ", ")
		}

		ib.injection.WriteTo(buf, insertMarkerAfterReturning)
	}

	return ib.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
}

// SetFlavor sets the flavor of compiled sql.
func (ib *InsertBuilder) SetFlavor(flavor Flavor) (old Flavor) {
	old = ib.args.Flavor
	ib.args.Flavor = flavor
	return
}

// Flavor returns flavor of builder
func (ib *InsertBuilder) Flavor() Flavor {
	return ib.args.Flavor
}

// Var returns a placeholder for value.
func (ib *InsertBuilder) Var(arg interface{}) string {
	return ib.args.Add(arg)
}

// SQL adds an arbitrary sql to current position.
func (ib *InsertBuilder) SQL(sql string) *InsertBuilder {
	ib.injection.SQL(ib.marker, sql)
	return ib
}
