// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"reflect"

	"github.com/huandu/go-clone"
)

const (
	unionDistinct = " UNION " // Default union type is DISTINCT.
	unionAll      = " UNION ALL "
)

const (
	unionMarkerInit injectionMarker = iota
	unionMarkerAfterUnion
	unionMarkerAfterOrderBy
	unionMarkerAfterLimit
)

// NewUnionBuilder creates a new UNION builder.
func NewUnionBuilder() *UnionBuilder {
	return DefaultFlavor.NewUnionBuilder()
}

func newUnionBuilder() *UnionBuilder {
	return &UnionBuilder{
		args:      &Args{},
		injection: newInjection(),
	}
}

// Clone returns a deep copy of UnionBuilder.
// It's useful when you want to create a base builder and clone it to build similar queries.
func (ub *UnionBuilder) Clone() *UnionBuilder {
	return clone.Clone(ub).(*UnionBuilder)
}

func init() {
	t := reflect.TypeOf(UnionBuilder{})
	clone.SetCustomFunc(t, func(allocator *clone.Allocator, old, new reflect.Value) {
		cloned := allocator.CloneSlowly(old)
		new.Set(cloned)

		ub := cloned.Addr().Interface().(*UnionBuilder)
		for i, b := range ub.builders {
			ub.args.Replace(ub.builderVars[i], b)
		}
	})
}

// UnionBuilder is a builder to build UNION.
type UnionBuilder struct {
	opt         string
	orderByCols []string
	order       string
	limitVar    string
	offsetVar   string

	builders    []Builder
	builderVars []string

	args *Args

	injection *injection
	marker    injectionMarker
}

var _ Builder = new(UnionBuilder)

// Union unions all builders together using UNION operator.
func Union(builders ...Builder) *UnionBuilder {
	return DefaultFlavor.NewUnionBuilder().Union(builders...)
}

// Union unions all builders together using UNION operator.
func (ub *UnionBuilder) Union(builders ...Builder) *UnionBuilder {
	return ub.union(unionDistinct, builders...)
}

// UnionAll unions all builders together using UNION ALL operator.
func UnionAll(builders ...Builder) *UnionBuilder {
	return DefaultFlavor.NewUnionBuilder().UnionAll(builders...)
}

// UnionAll unions all builders together using UNION ALL operator.
func (ub *UnionBuilder) UnionAll(builders ...Builder) *UnionBuilder {
	return ub.union(unionAll, builders...)
}

func (ub *UnionBuilder) union(opt string, builders ...Builder) *UnionBuilder {
	builderVars := make([]string, 0, len(builders))

	for _, b := range builders {
		builderVars = append(builderVars, ub.Var(b))
	}

	ub.opt = opt
	ub.builders = append([]Builder(nil), builders...)
	ub.builderVars = builderVars
	ub.marker = unionMarkerAfterUnion
	return ub
}

// OrderBy sets columns of ORDER BY in SELECT.
func (ub *UnionBuilder) OrderBy(col ...string) *UnionBuilder {
	ub.orderByCols = col
	ub.marker = unionMarkerAfterOrderBy
	return ub
}

// Asc sets order of ORDER BY to ASC.
func (ub *UnionBuilder) Asc() *UnionBuilder {
	ub.order = "ASC"
	ub.marker = unionMarkerAfterOrderBy
	return ub
}

// Desc sets order of ORDER BY to DESC.
func (ub *UnionBuilder) Desc() *UnionBuilder {
	ub.order = "DESC"
	ub.marker = unionMarkerAfterOrderBy
	return ub
}

// Limit sets the LIMIT in SELECT.
func (ub *UnionBuilder) Limit(limit int) *UnionBuilder {
	if limit < 0 {
		ub.limitVar = ""
		return ub
	}

	ub.limitVar = ub.Var(limit)
	ub.marker = unionMarkerAfterLimit
	return ub
}

// Offset sets the LIMIT offset in SELECT.
func (ub *UnionBuilder) Offset(offset int) *UnionBuilder {
	if offset < 0 {
		ub.offsetVar = ""
		return ub
	}

	ub.offsetVar = ub.Var(offset)
	ub.marker = unionMarkerAfterLimit
	return ub
}

// String returns the compiled SELECT string.
func (ub *UnionBuilder) String() string {
	s, _ := ub.Build()
	return s
}

// Build returns compiled SELECT string and args.
// They can be used in `DB#Query` of package `database/sql` directly.
func (ub *UnionBuilder) Build() (sql string, args []interface{}) {
	return ub.BuildWithFlavor(ub.args.Flavor)
}

// BuildWithFlavor returns compiled SELECT string and args with flavor and initial args.
// They can be used in `DB#Query` of package `database/sql` directly.
func (ub *UnionBuilder) BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	buf := newStringBuilder()
	ub.injection.WriteTo(buf, unionMarkerInit)

	nestedSelect := (flavor == Oracle && (len(ub.limitVar) > 0 || len(ub.offsetVar) > 0)) ||
		(flavor == Informix && len(ub.limitVar) > 0)

	if len(ub.builderVars) > 0 {
		needParen := flavor != SQLite

		if nestedSelect {
			buf.WriteLeadingString("SELECT * FROM (")
		}

		if needParen {
			buf.WriteLeadingString("(")
			buf.WriteString(ub.builderVars[0])
			buf.WriteRune(')')
		} else {
			buf.WriteLeadingString(ub.builderVars[0])
		}

		for _, b := range ub.builderVars[1:] {
			buf.WriteString(ub.opt)

			if needParen {
				buf.WriteRune('(')
			}

			buf.WriteString(b)

			if needParen {
				buf.WriteRune(')')
			}
		}

		if nestedSelect {
			buf.WriteLeadingString(")")
		}
	}

	ub.injection.WriteTo(buf, unionMarkerAfterUnion)

	if len(ub.orderByCols) > 0 {
		buf.WriteLeadingString("ORDER BY ")
		buf.WriteStrings(ub.orderByCols, ", ")

		if ub.order != "" {
			buf.WriteRune(' ')
			buf.WriteString(ub.order)
		}

		ub.injection.WriteTo(buf, unionMarkerAfterOrderBy)
	}

	switch flavor {
	case MySQL, SQLite, ClickHouse:
		if len(ub.limitVar) > 0 {
			buf.WriteLeadingString("LIMIT ")
			buf.WriteString(ub.limitVar)

			if len(ub.offsetVar) > 0 {
				buf.WriteLeadingString("OFFSET ")
				buf.WriteString(ub.offsetVar)
			}
		}

	case CQL:
		if len(ub.limitVar) > 0 {
			buf.WriteLeadingString("LIMIT ")
			buf.WriteString(ub.limitVar)
		}

	case PostgreSQL:
		if len(ub.limitVar) > 0 {
			buf.WriteLeadingString("LIMIT ")
			buf.WriteString(ub.limitVar)
		}

		if len(ub.offsetVar) > 0 {
			buf.WriteLeadingString("OFFSET ")
			buf.WriteString(ub.offsetVar)
		}

	case Presto:
		// There might be a hidden constraint in Presto requiring offset to be set before limit.
		// The select statement documentation (https://prestodb.io/docs/current/sql/select.html)
		// puts offset before limit, and Trino, which is based on Presto, seems
		// to require this specific order.
		if len(ub.offsetVar) > 0 {
			buf.WriteLeadingString("OFFSET ")
			buf.WriteString(ub.offsetVar)
		}

		if len(ub.limitVar) > 0 {
			buf.WriteLeadingString("LIMIT ")
			buf.WriteString(ub.limitVar)
		}

	case SQLServer:
		// If ORDER BY is not set, sort column #1 by default.
		// It's required to make OFFSET...FETCH work.
		if len(ub.orderByCols) == 0 && (len(ub.limitVar) > 0 || len(ub.offsetVar) > 0) {
			buf.WriteLeadingString("ORDER BY 1")
		}

		if len(ub.offsetVar) > 0 {
			buf.WriteLeadingString("OFFSET ")
			buf.WriteString(ub.offsetVar)
			buf.WriteString(" ROWS")
		}

		if len(ub.limitVar) > 0 {
			if len(ub.offsetVar) == 0 {
				buf.WriteLeadingString("OFFSET 0 ROWS")
			}

			buf.WriteLeadingString("FETCH NEXT ")
			buf.WriteString(ub.limitVar)
			buf.WriteString(" ROWS ONLY")
		}

	case Oracle:
		// It's required to make OFFSET...FETCH work.
		if len(ub.offsetVar) > 0 {
			buf.WriteLeadingString("OFFSET ")
			buf.WriteString(ub.offsetVar)
			buf.WriteString(" ROWS")
		}

		if len(ub.limitVar) > 0 {
			if len(ub.offsetVar) == 0 {
				buf.WriteLeadingString("OFFSET 0 ROWS")
			}

			buf.WriteLeadingString("FETCH NEXT ")
			buf.WriteString(ub.limitVar)
			buf.WriteString(" ROWS ONLY")
		}

	case Informix:
		// [SKIP N] FIRST M
		// M must be greater than 0
		if len(ub.limitVar) > 0 {
			if len(ub.offsetVar) > 0 {
				buf.WriteLeadingString("SKIP ")
				buf.WriteString(ub.offsetVar)
			}

			buf.WriteLeadingString("FIRST ")
			buf.WriteString(ub.limitVar)
		}

	case Doris:
		// #192: Doris doesn't support ? in OFFSET and LIMIT.
		if len(ub.limitVar) > 0 {
			buf.WriteLeadingString("LIMIT ")
			buf.WriteString(fmt.Sprint(ub.args.Value(ub.limitVar)))

			if len(ub.offsetVar) > 0 {
				buf.WriteLeadingString("OFFSET ")
				buf.WriteString(fmt.Sprint(ub.args.Value(ub.offsetVar)))
			}
		}
	}

	if len(ub.limitVar) > 0 {
		ub.injection.WriteTo(buf, unionMarkerAfterLimit)
	}

	return ub.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
}

// SetFlavor sets the flavor of compiled sql.
func (ub *UnionBuilder) SetFlavor(flavor Flavor) (old Flavor) {
	old = ub.args.Flavor
	ub.args.Flavor = flavor
	return
}

// Flavor returns flavor of builder
func (ub *UnionBuilder) Flavor() Flavor {
	return ub.args.Flavor
}

// Var returns a placeholder for value.
func (ub *UnionBuilder) Var(arg interface{}) string {
	return ub.args.Add(arg)
}

// SQL adds an arbitrary sql to current position.
func (ub *UnionBuilder) SQL(sql string) *UnionBuilder {
	ub.injection.SQL(ub.marker, sql)
	return ub
}
