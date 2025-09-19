// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

// WhereClause is a Builder for WHERE clause.
// All builders which support `WHERE` clause have an anonymous `WhereClause` field,
// in which the conditions are stored.
//
// WhereClause can be shared among multiple builders.
// However, it is not thread-safe.
type WhereClause struct {
	flavor  Flavor
	clauses []clause
}

var _ Builder = new(WhereClause)

// NewWhereClause creates a new WhereClause.
func NewWhereClause() *WhereClause {
	return &WhereClause{}
}

// CopyWhereClause creates a copy of the whereClause.
func CopyWhereClause(whereClause *WhereClause) *WhereClause {
	clauses := make([]clause, len(whereClause.clauses))
	copy(clauses, whereClause.clauses)

	return &WhereClause{
		flavor:  whereClause.flavor,
		clauses: clauses,
	}
}

type clause struct {
	args     *Args
	andExprs []string
}

func (c *clause) Build(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	exprs := filterEmptyStrings(c.andExprs)

	if len(exprs) == 0 {
		return
	}

	buf := newStringBuilder()
	buf.WriteStrings(exprs, " AND ")
	sql, args = c.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
	return
}

// whereClauseProxy is a proxy for WhereClause.
// It's useful when the WhereClause in a build can be changed.
type whereClauseProxy struct {
	*WhereClause
}

var _ Builder = new(whereClauseProxy)

// BuildWithFlavor builds a WHERE clause with the specified flavor and initial arguments.
func (wc *WhereClause) BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	if wc == nil || len(wc.clauses) == 0 {
		return "", nil
	}

	buf := newStringBuilder()
	buf.WriteLeadingString("WHERE ")

	sql, args = wc.clauses[0].Build(flavor, initialArg...)
	buf.WriteString(sql)

	for _, clause := range wc.clauses[1:] {
		buf.WriteString(" AND ")
		sql, args = clause.Build(flavor, args...)
		buf.WriteString(sql)
	}

	return buf.String(), args
}

// Build returns compiled WHERE clause string and args.
func (wc *WhereClause) Build() (sql string, args []interface{}) {
	return wc.BuildWithFlavor(wc.flavor)
}

// SetFlavor sets the flavor of compiled sql.
// When the WhereClause belongs to a builder, the flavor of the builder will be used when building SQL.
func (wc *WhereClause) SetFlavor(flavor Flavor) (old Flavor) {
	old = wc.flavor
	wc.flavor = flavor
	return
}

// Flavor returns flavor of clause
func (wc *WhereClause) Flavor() Flavor {
	return wc.flavor
}

// AddWhereExpr adds an AND expression to WHERE clause with the specified arguments.
func (wc *WhereClause) AddWhereExpr(args *Args, andExpr ...string) *WhereClause {
	if len(andExpr) == 0 {
		return wc
	}

	andExprsBytesLen := estimateStringsBytes(andExpr)

	if andExprsBytesLen == 0 {
		return wc
	}

	// Merge with last clause if possible.
	if len(wc.clauses) > 0 {
		lastClause := &wc.clauses[len(wc.clauses)-1]

		if lastClause.args == args {
			lastClause.andExprs = append(lastClause.andExprs, andExpr...)
			return wc
		}
	}

	wc.clauses = append(wc.clauses, clause{
		args:     args,
		andExprs: andExpr,
	})
	return wc
}

// AddWhereClause adds all clauses in the whereClause to the wc.
func (wc *WhereClause) AddWhereClause(whereClause *WhereClause) *WhereClause {
	if wc == nil {
		return nil
	}
	if whereClause == nil {
		return wc
	}

	wc.clauses = append(wc.clauses, whereClause.clauses...)
	return wc
}
