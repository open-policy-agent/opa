// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/huandu/go-clone"
)

const (
	selectMarkerInit injectionMarker = iota
	selectMarkerAfterWith
	selectMarkerAfterSelect
	selectMarkerAfterFrom
	selectMarkerAfterJoin
	selectMarkerAfterWhere
	selectMarkerAfterGroupBy
	selectMarkerAfterOrderBy
	selectMarkerAfterLimit
	selectMarkerAfterFor
)

// JoinOption is the option in JOIN.
type JoinOption string

// Join options.
const (
	FullJoin       JoinOption = "FULL"
	FullOuterJoin  JoinOption = "FULL OUTER"
	InnerJoin      JoinOption = "INNER"
	LeftJoin       JoinOption = "LEFT"
	LeftOuterJoin  JoinOption = "LEFT OUTER"
	RightJoin      JoinOption = "RIGHT"
	RightOuterJoin JoinOption = "RIGHT OUTER"
)

// NewSelectBuilder creates a new SELECT builder.
func NewSelectBuilder() *SelectBuilder {
	return DefaultFlavor.NewSelectBuilder()
}

func newSelectBuilder() *SelectBuilder {
	args := &Args{}
	proxy := &whereClauseProxy{}
	return &SelectBuilder{
		whereClauseProxy: proxy,
		whereClauseExpr:  args.Add(proxy),

		Cond: Cond{
			Args: args,
		},
		args:      args,
		injection: newInjection(),
	}
}

// Clone returns a deep copy of SelectBuilder.
// It's useful when you want to create a base builder and clone it to build similar queries.
func (sb *SelectBuilder) Clone() *SelectBuilder {
	return clone.Clone(sb).(*SelectBuilder)
}

func init() {
	t := reflect.TypeOf(SelectBuilder{})
	clone.SetCustomFunc(t, func(allocator *clone.Allocator, old, new reflect.Value) {
		cloned := allocator.CloneSlowly(old)
		new.Set(cloned)

		sb := cloned.Addr().Interface().(*SelectBuilder)
		sb.args.Replace(sb.whereClauseExpr, sb.whereClauseProxy)
		sb.args.Replace(sb.cteBuilderVar, sb.cteBuilder)
	})
}

// SelectBuilder is a builder to build SELECT.
type SelectBuilder struct {
	*WhereClause
	Cond

	whereClauseProxy *whereClauseProxy
	whereClauseExpr  string

	cteBuilderVar string
	cteBuilder    *CTEBuilder

	distinct    bool
	tables      []string
	selectCols  []string
	joinOptions []JoinOption
	joinTables  []string
	joinExprs   [][]string
	havingExprs []string
	groupByCols []string
	orderByCols []string
	order       string
	limitVar    string
	offsetVar   string
	forWhat     string

	args *Args

	injection *injection
	marker    injectionMarker
}

var _ Builder = new(SelectBuilder)

// Select sets columns in SELECT.
func Select(col ...string) *SelectBuilder {
	return DefaultFlavor.NewSelectBuilder().Select(col...)
}

// TableNames returns all table names in this SELECT statement.
func (sb *SelectBuilder) TableNames() []string {
	var additionalTableNames []string

	if sb.cteBuilder != nil {
		additionalTableNames = sb.cteBuilder.tableNamesForFrom()
	}

	var tableNames []string

	if len(sb.tables) > 0 && len(additionalTableNames) > 0 {
		tableNames = make([]string, len(sb.tables)+len(additionalTableNames))
		copy(tableNames, sb.tables)
		copy(tableNames[len(sb.tables):], additionalTableNames)
	} else if len(sb.tables) > 0 {
		tableNames = sb.tables
	} else if len(additionalTableNames) > 0 {
		tableNames = additionalTableNames
	}

	return tableNames
}

// With sets WITH clause (the Common Table Expression) before SELECT.
func (sb *SelectBuilder) With(builder *CTEBuilder) *SelectBuilder {
	sb.marker = selectMarkerAfterWith
	sb.cteBuilderVar = sb.Var(builder)
	sb.cteBuilder = builder
	return sb
}

// Select sets columns in SELECT.
func (sb *SelectBuilder) Select(col ...string) *SelectBuilder {
	sb.selectCols = col
	sb.marker = selectMarkerAfterSelect
	return sb
}

// SelectMore adds more columns in SELECT.
func (sb *SelectBuilder) SelectMore(col ...string) *SelectBuilder {
	sb.selectCols = append(sb.selectCols, col...)
	sb.marker = selectMarkerAfterSelect
	return sb
}

// Distinct marks this SELECT as DISTINCT.
func (sb *SelectBuilder) Distinct() *SelectBuilder {
	sb.distinct = true
	sb.marker = selectMarkerAfterSelect
	return sb
}

// From sets table names in SELECT.
func (sb *SelectBuilder) From(table ...string) *SelectBuilder {
	sb.tables = table
	sb.marker = selectMarkerAfterFrom
	return sb
}

// Join sets expressions of JOIN in SELECT.
//
// It builds a JOIN expression like
//
//	JOIN table ON onExpr[0] AND onExpr[1] ...
func (sb *SelectBuilder) Join(table string, onExpr ...string) *SelectBuilder {
	sb.marker = selectMarkerAfterJoin
	return sb.JoinWithOption("", table, onExpr...)
}

// JoinWithOption sets expressions of JOIN with an option.
//
// It builds a JOIN expression like
//
//	option JOIN table ON onExpr[0] AND onExpr[1] ...
//
// Here is a list of supported options.
//   - FullJoin: FULL JOIN
//   - FullOuterJoin: FULL OUTER JOIN
//   - InnerJoin: INNER JOIN
//   - LeftJoin: LEFT JOIN
//   - LeftOuterJoin: LEFT OUTER JOIN
//   - RightJoin: RIGHT JOIN
//   - RightOuterJoin: RIGHT OUTER JOIN
func (sb *SelectBuilder) JoinWithOption(option JoinOption, table string, onExpr ...string) *SelectBuilder {
	sb.joinOptions = append(sb.joinOptions, option)
	sb.joinTables = append(sb.joinTables, table)
	sb.joinExprs = append(sb.joinExprs, onExpr)
	sb.marker = selectMarkerAfterJoin
	return sb
}

// Where sets expressions of WHERE in SELECT.
func (sb *SelectBuilder) Where(andExpr ...string) *SelectBuilder {
	if len(andExpr) == 0 || estimateStringsBytes(andExpr) == 0 {
		return sb
	}

	if sb.WhereClause == nil {
		sb.WhereClause = NewWhereClause()
	}

	sb.WhereClause.AddWhereExpr(sb.args, andExpr...)
	sb.marker = selectMarkerAfterWhere
	return sb
}

// AddWhereClause adds all clauses in the whereClause to SELECT.
func (sb *SelectBuilder) AddWhereClause(whereClause *WhereClause) *SelectBuilder {
	if sb.WhereClause == nil {
		sb.WhereClause = NewWhereClause()
	}

	sb.WhereClause.AddWhereClause(whereClause)
	return sb
}

// Having sets expressions of HAVING in SELECT.
func (sb *SelectBuilder) Having(andExpr ...string) *SelectBuilder {
	sb.havingExprs = append(sb.havingExprs, andExpr...)
	sb.marker = selectMarkerAfterGroupBy
	return sb
}

// GroupBy sets columns of GROUP BY in SELECT.
func (sb *SelectBuilder) GroupBy(col ...string) *SelectBuilder {
	sb.groupByCols = append(sb.groupByCols, col...)
	sb.marker = selectMarkerAfterGroupBy
	return sb
}

// OrderBy sets columns of ORDER BY in SELECT.
func (sb *SelectBuilder) OrderBy(col ...string) *SelectBuilder {
	sb.orderByCols = append(sb.orderByCols, col...)
	sb.marker = selectMarkerAfterOrderBy
	return sb
}

// Asc sets order of ORDER BY to ASC.
func (sb *SelectBuilder) Asc() *SelectBuilder {
	sb.order = "ASC"
	sb.marker = selectMarkerAfterOrderBy
	return sb
}

// Desc sets order of ORDER BY to DESC.
func (sb *SelectBuilder) Desc() *SelectBuilder {
	sb.order = "DESC"
	sb.marker = selectMarkerAfterOrderBy
	return sb
}

// Limit sets the LIMIT in SELECT.
func (sb *SelectBuilder) Limit(limit int) *SelectBuilder {
	if limit < 0 {
		sb.limitVar = ""
		return sb
	}

	sb.limitVar = sb.Var(limit)
	sb.marker = selectMarkerAfterLimit
	return sb
}

// Offset sets the LIMIT offset in SELECT.
func (sb *SelectBuilder) Offset(offset int) *SelectBuilder {
	if offset < 0 {
		sb.offsetVar = ""
		return sb
	}

	sb.offsetVar = sb.Var(offset)
	sb.marker = selectMarkerAfterLimit
	return sb
}

// ForUpdate adds FOR UPDATE at the end of SELECT statement.
func (sb *SelectBuilder) ForUpdate() *SelectBuilder {
	sb.forWhat = "UPDATE"
	sb.marker = selectMarkerAfterFor
	return sb
}

// ForShare adds FOR SHARE at the end of SELECT statement.
func (sb *SelectBuilder) ForShare() *SelectBuilder {
	sb.forWhat = "SHARE"
	sb.marker = selectMarkerAfterFor
	return sb
}

// As returns an AS expression.
func (sb *SelectBuilder) As(name, alias string) string {
	return fmt.Sprintf("%s AS %s", name, alias)
}

// BuilderAs returns an AS expression wrapping a complex SQL.
// According to SQL syntax, SQL built by builder is surrounded by parens.
func (sb *SelectBuilder) BuilderAs(builder Builder, alias string) string {
	return fmt.Sprintf("(%s) AS %s", sb.Var(builder), alias)
}

// LateralAs returns a LATERAL derived table expression wrapping a complex SQL.
func (sb *SelectBuilder) LateralAs(builder Builder, alias string) string {
	return fmt.Sprintf("LATERAL (%s) AS %s", sb.Var(builder), alias)
}

// NumCol returns the number of columns to select.
func (sb *SelectBuilder) NumCol() int {
	return len(sb.selectCols)
}

// String returns the compiled SELECT string.
func (sb *SelectBuilder) String() string {
	s, _ := sb.Build()
	return s
}

// Build returns compiled SELECT string and args.
// They can be used in `DB#Query` of package `database/sql` directly.
func (sb *SelectBuilder) Build() (sql string, args []interface{}) {
	return sb.BuildWithFlavor(sb.args.Flavor)
}

// BuildWithFlavor returns compiled SELECT string and args with flavor and initial args.
// They can be used in `DB#Query` of package `database/sql` directly.
func (sb *SelectBuilder) BuildWithFlavor(flavor Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	buf := newStringBuilder()
	sb.injection.WriteTo(buf, selectMarkerInit)

	oraclePage := flavor == Oracle && (len(sb.limitVar) > 0 || len(sb.offsetVar) > 0)

	if sb.cteBuilderVar != "" {
		buf.WriteLeadingString(sb.cteBuilderVar)
		sb.injection.WriteTo(buf, selectMarkerAfterWith)
	}

	if len(sb.selectCols) > 0 {
		buf.WriteLeadingString("SELECT ")

		if sb.distinct {
			buf.WriteString("DISTINCT ")
		}

		if oraclePage {
			var selectCols = make([]string, 0, len(sb.selectCols))
			for i := range sb.selectCols {
				cols := strings.SplitN(sb.selectCols[i], ".", 2)

				if len(cols) == 1 {
					selectCols = append(selectCols, cols[0])
				} else {
					selectCols = append(selectCols, cols[1])
				}
			}
			buf.WriteStrings(selectCols, ", ")
		} else {
			buf.WriteStrings(sb.selectCols, ", ")
		}
	}

	sb.injection.WriteTo(buf, selectMarkerAfterSelect)

	if oraclePage {
		if len(sb.selectCols) > 0 {
			buf.WriteLeadingString("FROM (SELECT ")

			if sb.distinct {
				buf.WriteString("DISTINCT ")
			}

			var selectCols = make([]string, 0, len(sb.selectCols)+1)
			selectCols = append(selectCols, "ROWNUM r")

			for i := range sb.selectCols {
				cols := strings.SplitN(sb.selectCols[i], ".", 2)
				if len(cols) == 1 {
					selectCols = append(selectCols, cols[0])
				} else {
					selectCols = append(selectCols, cols[1])
				}
			}

			buf.WriteStrings(selectCols, ", ")
			buf.WriteLeadingString("FROM (SELECT ")
			buf.WriteStrings(sb.selectCols, ", ")
		}
	}

	tableNames := sb.TableNames()

	if len(tableNames) > 0 {
		buf.WriteLeadingString("FROM ")
		buf.WriteStrings(tableNames, ", ")
	}

	sb.injection.WriteTo(buf, selectMarkerAfterFrom)

	for i := range sb.joinTables {
		if option := sb.joinOptions[i]; option != "" {
			buf.WriteLeadingString(string(option))
		}

		buf.WriteLeadingString("JOIN ")
		buf.WriteString(sb.joinTables[i])

		if exprs := filterEmptyStrings(sb.joinExprs[i]); len(exprs) > 0 {
			buf.WriteString(" ON ")
			buf.WriteStrings(exprs, " AND ")
		}
	}

	if len(sb.joinTables) > 0 {
		sb.injection.WriteTo(buf, selectMarkerAfterJoin)
	}

	if sb.WhereClause != nil {
		sb.whereClauseProxy.WhereClause = sb.WhereClause
		defer func() {
			sb.whereClauseProxy.WhereClause = nil
		}()

		buf.WriteLeadingString(sb.whereClauseExpr)
		sb.injection.WriteTo(buf, selectMarkerAfterWhere)
	}

	if len(sb.groupByCols) > 0 {
		buf.WriteLeadingString("GROUP BY ")
		buf.WriteStrings(sb.groupByCols, ", ")

		if havingExprs := filterEmptyStrings(sb.havingExprs); len(havingExprs) > 0 {
			buf.WriteString(" HAVING ")
			buf.WriteStrings(havingExprs, " AND ")
		}

		sb.injection.WriteTo(buf, selectMarkerAfterGroupBy)
	}

	if len(sb.orderByCols) > 0 {
		buf.WriteLeadingString("ORDER BY ")
		buf.WriteStrings(sb.orderByCols, ", ")

		if sb.order != "" {
			buf.WriteRune(' ')
			buf.WriteString(sb.order)
		}

		sb.injection.WriteTo(buf, selectMarkerAfterOrderBy)
	}

	switch flavor {
	case MySQL, SQLite, ClickHouse:
		if len(sb.limitVar) > 0 {
			buf.WriteLeadingString("LIMIT ")
			buf.WriteString(sb.limitVar)

			if len(sb.offsetVar) > 0 {
				buf.WriteLeadingString("OFFSET ")
				buf.WriteString(sb.offsetVar)
			}
		}

	case CQL:
		if len(sb.limitVar) > 0 {
			buf.WriteLeadingString("LIMIT ")
			buf.WriteString(sb.limitVar)
		}

	case PostgreSQL:
		if len(sb.limitVar) > 0 {
			buf.WriteLeadingString("LIMIT ")
			buf.WriteString(sb.limitVar)
		}

		if len(sb.offsetVar) > 0 {
			buf.WriteLeadingString("OFFSET ")
			buf.WriteString(sb.offsetVar)
		}

	case Presto:
		// There might be a hidden constraint in Presto requiring offset to be set before limit.
		// The select statement documentation (https://prestodb.io/docs/current/sql/select.html)
		// puts offset before limit, and Trino, which is based on Presto, seems
		// to require this specific order.
		if len(sb.offsetVar) > 0 {
			buf.WriteLeadingString("OFFSET ")
			buf.WriteString(sb.offsetVar)
		}

		if len(sb.limitVar) > 0 {
			buf.WriteLeadingString("LIMIT ")
			buf.WriteString(sb.limitVar)
		}

	case SQLServer:
		// If ORDER BY is not set, sort column #1 by default.
		// It's required to make OFFSET...FETCH work.
		if len(sb.orderByCols) == 0 && (len(sb.limitVar) > 0 || len(sb.offsetVar) > 0) {
			buf.WriteLeadingString("ORDER BY 1")
		}

		if len(sb.offsetVar) > 0 {
			buf.WriteLeadingString("OFFSET ")
			buf.WriteString(sb.offsetVar)
			buf.WriteString(" ROWS")
		}

		if len(sb.limitVar) > 0 {
			if len(sb.offsetVar) == 0 {
				buf.WriteLeadingString("OFFSET 0 ROWS")
			}

			buf.WriteLeadingString("FETCH NEXT ")
			buf.WriteString(sb.limitVar)
			buf.WriteString(" ROWS ONLY")
		}

	case Oracle:
		if oraclePage {
			buf.WriteString(") ")

			if len(sb.tables) > 0 {
				buf.WriteStrings(sb.tables, ", ")
			}

			buf.WriteString(") WHERE ")

			if len(sb.limitVar) > 0 {
				buf.WriteString("r BETWEEN ")

				if len(sb.offsetVar) > 0 {
					buf.WriteString(sb.offsetVar)
					buf.WriteString(" + 1 AND ")
					buf.WriteString(sb.limitVar)
					buf.WriteString(" + ")
					buf.WriteString(sb.offsetVar)
				} else {
					buf.WriteString("1 AND ")
					buf.WriteString(sb.limitVar)
					buf.WriteString(" + 1")
				}
			} else {
				// As oraclePage is true, sb.offsetVar must not be empty.
				buf.WriteString("r >= ")
				buf.WriteString(sb.offsetVar)
				buf.WriteString(" + 1")
			}
		}

	case Informix:
		// [SKIP N] FIRST M
		// M must be greater than 0
		if len(sb.limitVar) > 0 {
			if len(sb.offsetVar) > 0 {
				buf.WriteLeadingString("SKIP ")
				buf.WriteString(sb.offsetVar)
			}

			buf.WriteLeadingString("FIRST ")
			buf.WriteString(sb.limitVar)
		}

	case Doris:
		// #192: Doris doesn't support ? in OFFSET and LIMIT.
		if len(sb.limitVar) > 0 {
			buf.WriteLeadingString("LIMIT ")
			buf.WriteString(fmt.Sprint(sb.args.Value(sb.limitVar)))

			if len(sb.offsetVar) > 0 {
				buf.WriteLeadingString("OFFSET ")
				buf.WriteString(fmt.Sprint(sb.args.Value(sb.offsetVar)))
			}
		}
	}

	if len(sb.limitVar) > 0 {
		sb.injection.WriteTo(buf, selectMarkerAfterLimit)
	}

	if sb.forWhat != "" {
		buf.WriteLeadingString("FOR ")
		buf.WriteString(sb.forWhat)

		sb.injection.WriteTo(buf, selectMarkerAfterFor)
	}

	return sb.args.CompileWithFlavor(buf.String(), flavor, initialArg...)
}

// SetFlavor sets the flavor of compiled sql.
func (sb *SelectBuilder) SetFlavor(flavor Flavor) (old Flavor) {
	old = sb.args.Flavor
	sb.args.Flavor = flavor
	return
}

// Flavor returns flavor of builder
func (sb *SelectBuilder) Flavor() Flavor {
	return sb.args.Flavor
}

// SQL adds an arbitrary sql to current position.
func (sb *SelectBuilder) SQL(sql string) *SelectBuilder {
	sb.injection.SQL(sb.marker, sql)
	return sb
}
