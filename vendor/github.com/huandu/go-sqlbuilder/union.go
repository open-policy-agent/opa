// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

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

// UnionBuilder is a builder to build UNION.
type UnionBuilder struct {
	opt         string
	builderVars []string
	orderByCols []string
	order       string
	limitVar    string
	offsetVar   string

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

	if len(ub.builderVars) > 0 {
		needParen := flavor != SQLite

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

	if len(ub.limitVar) > 0 {
		buf.WriteLeadingString("LIMIT ")
		buf.WriteString(ub.limitVar)

	}

	if ((MySQL == flavor || Informix == flavor) && len(ub.limitVar) > 0) || PostgreSQL == flavor {
		if len(ub.offsetVar) > 0 {
			buf.WriteLeadingString("OFFSET ")
			buf.WriteString(ub.offsetVar)
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
