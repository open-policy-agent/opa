// Copyright 2025 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package ucast

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/huandu/go-sqlbuilder"
)

// The "union" structure for incoming UCAST trees.
type UCASTNode struct {
	Type  string `json:"type"`
	Op    string `json:"operator"`
	Field string `json:"field,omitempty"`
	Value any    `json:"value,omitempty"`
}

// FieldRef can be used in UCASTNode.Value to reference another field, i.e. database column.
type FieldRef struct {
	Field string `json:"field"`
}

// Null represents a NULL value in a SQL query. We need our own type
// to control both the JSON marshalling and the SQL generation.
type Null struct{}

func (Null) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

var (
	compoundOps = []string{"and", "or", "not"}
	documentOps = []string{"exists"}
	fieldOps    = []string{"eq", "ne", "gt", "lt", "ge", "le", "gte", "lte", "in"} // "nin"
)

func dialectToFlavor(dialect string) sqlbuilder.Flavor {
	switch dialect {
	case "mysql":
		return sqlbuilder.MySQL
	case "sqlite", "sqlite-internal":
		return sqlbuilder.SQLite
	case "postgres", "postgresql":
		return sqlbuilder.PostgreSQL
	case "sqlserver":
		return sqlbuilder.SQLServer
	default:
		return sqlbuilder.SQLite
	}
}

func interpolateByDialect(dialect string, s string, args []any) (string, error) {
	return dialectToFlavor(dialect).Interpolate(s, args)
}

func (u *UCASTNode) AsSQL(dialect string) (string, error) {
	// Build up the SQL expression using the UCASTNode tree.
	cond := sqlbuilder.NewCond()
	where := sqlbuilder.NewWhereClause()
	conditionStr, err := u.asSQL(cond, dialect)
	if err != nil {
		return "", err
	}
	where.AddWhereExpr(cond.Args, conditionStr)
	s, args := where.BuildWithFlavor(dialectToFlavor(dialect))
	// Interpolate in the arguments into the SQL string.
	return interpolateByDialect(dialect, s, args)
}

// Uses our SQL generator library to build up a larger SQL expression.
func (u *UCASTNode) asSQL(cond *sqlbuilder.Cond, dialect string) (string, error) {
	cond.Args.Flavor = dialectToFlavor(dialect)
	uType := u.Type
	operator := u.Op
	field := u.Field
	value := u.Value

	switch {
	case slices.Contains(fieldOps, operator) || uType == "field":
		switch value {
		case nil:
			return "", nil
		case Null{}:
			switch operator {
			case "eq":
				return cond.IsNull(field), nil
			case "ne":
				return cond.IsNotNull(field), nil
			default:
				return "", errors.New("null value can only be used with 'eq' or 'ne' operators")
			}
		default:
			if fr, ok := value.(FieldRef); ok {
				value = sqlbuilder.Raw(fr.Field)
			}
		}
		switch operator {
		case "eq":
			return cond.Equal(field, value), nil
		case "ne":
			return cond.NotEqual(field, value), nil
		case "gt":
			return cond.GreaterThan(field, value), nil
		case "lt":
			return cond.LessThan(field, value), nil
		case "ge", "gte":
			return cond.GreaterEqualThan(field, value), nil
		case "le", "lte":
			return cond.LessEqualThan(field, value), nil
		case "in":
			if arr, ok := (value).([]any); ok {
				return cond.In(field, arr...), nil
			}
			return "", errors.New("field operator 'in' requires collection argument")
		case "startswith":
			if dialect == "sqlite-internal" {
				return cond.Var(sqlbuilder.Build("internal_startswith($?, $?)", sqlbuilder.Raw(field), value)), nil
			}
			pattern, err := prefix(value)
			if err != nil {
				return "", err
			}
			return cond.Like(field, pattern), nil
		case "endswith":
			if dialect == "sqlite-internal" {
				return cond.Var(sqlbuilder.Build("internal_endswith($?, $?)", sqlbuilder.Raw(field), value)), nil
			}
			pattern, err := suffix(value)
			if err != nil {
				return "", err
			}
			return cond.Like(field, pattern), nil
		case "contains":
			if dialect == "sqlite-internal" {
				return cond.Var(sqlbuilder.Build("internal_contains($?, $?)", sqlbuilder.Raw(field), value)), nil
			}
			pattern, err := infix(value)
			if err != nil {
				return "", err
			}
			return cond.Like(field, pattern), nil

		default:
			return "", fmt.Errorf("unrecognized operator: %s", operator)
		}
	case slices.Contains(documentOps, operator) || uType == "document":
		// Note: We should add unary operations under this case, like NOT.
		if value == nil {
			return "", errors.New("document expression 'exists' requires a value")
		}
		if operator == "exists" {
			return cond.Exists(value), nil
		}
		return "", fmt.Errorf("unrecognized operator: %s", operator)
	case slices.Contains(compoundOps, operator) || uType == "compound":
		switch operator {
		case "and":
			if value == nil {
				return "", errors.New("compound expression 'and' requires a value")
			}
			if values, ok := (value).([]UCASTNode); ok {
				conds := make([]string, 0, len(values))
				for _, c := range values {
					condition, err := c.asSQL(cond, dialect)
					if err != nil {
						return "", err
					}
					conds = append(conds, condition)
				}
				return cond.And(conds...), nil
			}
			return "", errors.New("value must be an array")
		case "or":
			if value == nil {
				return "", errors.New("compound expression 'or' requires a value")
			}
			if values, ok := (value).([]UCASTNode); ok {
				conds := make([]string, 0, len(values))
				for _, c := range values {
					condition, err := c.asSQL(cond, dialect)
					if err != nil {
						return "", err
					}
					conds = append(conds, condition)
				}
				return cond.Or(conds...), nil
			}
			return "", errors.New("value must be an array")
		case "not":
			if value == nil {
				return "", errors.New("compound expression 'not' requires exactly one value")
			}
			node, ok := (value).([]UCASTNode)
			if ok {
				if len(node) != 1 {
					return "", errors.New("compound expression 'not' requires exactly one value")
				}
				condition, err := node[0].asSQL(cond, dialect)
				if err != nil {
					return "", err
				}
				return cond.Not(condition), nil
			}
			return "", fmt.Errorf("value must be a ucast node, got %T: %[1]v", value)
		}
		return "", fmt.Errorf("unrecognized operator: %s", operator)
	default:
		return "", fmt.Errorf("unrecognized operator: %s", operator)
	}
}

func prefix(p any) (string, error) {
	p0, ok := p.(string)
	if !ok {
		return "", fmt.Errorf("'startswith' pattern requires string argument, got %v %[1]T", p)
	}
	return escaped(p0) + "%", nil
}

func suffix(p any) (string, error) {
	p0, ok := p.(string)
	if !ok {
		return "", fmt.Errorf("'endswith' pattern requires string argument, got %v %[1]T", p)
	}
	return "%" + escaped(p0), nil
}

func infix(p any) (string, error) {
	p0, ok := p.(string)
	if !ok {
		return "", fmt.Errorf("'contains' pattern requires string argument, got %v %[1]T", p)
	}
	return "%" + escaped(p0) + "%", nil
}

func escaped(p0 string) string {
	p0 = strings.ReplaceAll(p0, `\`, `\\`)
	p0 = strings.ReplaceAll(p0, "_", `\_`)
	p0 = strings.ReplaceAll(p0, "%", `\%`)
	return p0
}
