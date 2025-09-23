// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"errors"
	"fmt"
)

// Supported flavors.
const (
	invalidFlavor Flavor = iota

	MySQL
	PostgreSQL
	SQLite
	SQLServer
	CQL
	ClickHouse
	Presto
	Oracle
	Informix
	Doris
)

var (
	// DefaultFlavor is the default flavor for all builders.
	DefaultFlavor = MySQL
)

var (
	// ErrInterpolateNotImplemented means the method or feature is not implemented right now.
	ErrInterpolateNotImplemented = errors.New("go-sqlbuilder: interpolation for this flavor is not implemented")

	// ErrInterpolateMissingArgs means there are some args missing in query, so it's not possible to
	// prepare a query with such args.
	ErrInterpolateMissingArgs = errors.New("go-sqlbuilder: not enough args when interpolating")

	// ErrInterpolateUnsupportedArgs means that some types of the args are not supported.
	ErrInterpolateUnsupportedArgs = errors.New("go-sqlbuilder: unsupported args when interpolating")
)

// Flavor is the flag to control the format of compiled sql.
type Flavor int

// String returns the name of f.
func (f Flavor) String() string {
	switch f {
	case MySQL:
		return "MySQL"
	case PostgreSQL:
		return "PostgreSQL"
	case SQLite:
		return "SQLite"
	case SQLServer:
		return "SQLServer"
	case CQL:
		return "CQL"
	case ClickHouse:
		return "ClickHouse"
	case Presto:
		return "Presto"
	case Oracle:
		return "Oracle"
	case Informix:
		return "Informix"
	case Doris:
		return "Doris"
	}

	return "<invalid>"
}

// Interpolate parses sql returned by `Args#Compile` or `Builder`,
// and interpolate args to replace placeholders in the sql.
//
// If there are some args missing in sql, e.g. the number of placeholders are larger than len(args),
// returns ErrMissingArgs error.
func (f Flavor) Interpolate(sql string, args []interface{}) (string, error) {
	switch f {
	case MySQL:
		return mysqlInterpolate(sql, args...)
	case PostgreSQL:
		return postgresqlInterpolate(sql, args...)
	case SQLite:
		return sqliteInterpolate(sql, args...)
	case SQLServer:
		return sqlserverInterpolate(sql, args...)
	case CQL:
		return cqlInterpolate(sql, args...)
	case ClickHouse:
		return clickhouseInterpolate(sql, args...)
	case Presto:
		return prestoInterpolate(sql, args...)
	case Oracle:
		return oracleInterpolate(sql, args...)
	case Informix:
		return informixInterpolate(sql, args...)
	case Doris:
		return dorisInterpolate(sql, args...)
	}

	return "", ErrInterpolateNotImplemented
}

// NewCreateTableBuilder creates a new CREATE TABLE builder with flavor.
func (f Flavor) NewCreateTableBuilder() *CreateTableBuilder {
	b := newCreateTableBuilder()
	b.SetFlavor(f)
	return b
}

// NewDeleteBuilder creates a new DELETE builder with flavor.
func (f Flavor) NewDeleteBuilder() *DeleteBuilder {
	b := newDeleteBuilder()
	b.SetFlavor(f)
	return b
}

// NewInsertBuilder creates a new INSERT builder with flavor.
func (f Flavor) NewInsertBuilder() *InsertBuilder {
	b := newInsertBuilder()
	b.SetFlavor(f)
	return b
}

// NewSelectBuilder creates a new SELECT builder with flavor.
func (f Flavor) NewSelectBuilder() *SelectBuilder {
	b := newSelectBuilder()
	b.SetFlavor(f)
	return b
}

// NewUpdateBuilder creates a new UPDATE builder with flavor.
func (f Flavor) NewUpdateBuilder() *UpdateBuilder {
	b := newUpdateBuilder()
	b.SetFlavor(f)
	return b
}

// NewUnionBuilder creates a new UNION builder with flavor.
func (f Flavor) NewUnionBuilder() *UnionBuilder {
	b := newUnionBuilder()
	b.SetFlavor(f)
	return b
}

// NewCTEBuilder creates a new CTE builder with flavor.
func (f Flavor) NewCTEBuilder() *CTEBuilder {
	b := newCTEBuilder()
	b.SetFlavor(f)
	return b
}

// NewCTETableBuilder creates a new CTE table builder with flavor.
func (f Flavor) NewCTEQueryBuilder() *CTEQueryBuilder {
	b := newCTEQueryBuilder()
	b.SetFlavor(f)
	return b
}

// Quote adds quote for name to make sure the name can be used safely
// as table name or field name.
//
//   - For MySQL, use back quote (`) to quote name;
//   - For PostgreSQL, SQL Server and SQLite, use double quote (") to quote name.
func (f Flavor) Quote(name string) string {
	switch f {
	case MySQL, ClickHouse, Doris:
		return fmt.Sprintf("`%s`", name)
	case PostgreSQL, SQLServer, SQLite, Presto, Oracle, Informix:
		return fmt.Sprintf(`"%s"`, name)
	case CQL:
		return fmt.Sprintf("'%s'", name)
	}

	return name
}

// PrepareInsertIgnore prepares the insert builder to build insert ignore SQL statement based on the sql flavor
func (f Flavor) PrepareInsertIgnore(table string, ib *InsertBuilder) {
	switch ib.args.Flavor {
	case MySQL, Oracle:
		ib.verb = "INSERT IGNORE"

	case PostgreSQL:
		// see https://www.postgresql.org/docs/current/sql-insert.html
		ib.verb = "INSERT"
		// add sql statement at the end after values, i.e. INSERT INTO ... ON CONFLICT DO NOTHING
		ib.marker = insertMarkerAfterValues
		ib.SQL("ON CONFLICT DO NOTHING")

	case SQLite:
		// see https://www.sqlite.org/lang_insert.html
		ib.verb = "INSERT OR IGNORE"

	case ClickHouse, CQL, SQLServer, Presto, Informix, Doris:
		// All other databases do not support insert ignore
		ib.verb = "INSERT"

	default:
		// panic if the db flavor is not supported
		panic(fmt.Errorf("unsupported db flavor: %s", ib.args.Flavor.String()))
	}

	// Set the table and reset the marker right after insert into
	ib.table = Escape(table)
	ib.marker = insertMarkerAfterInsertInto
}
