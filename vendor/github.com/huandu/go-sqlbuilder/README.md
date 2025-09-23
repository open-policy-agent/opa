# SQL builder for Go

[![Go](https://github.com/huandu/go-sqlbuilder/workflows/Go/badge.svg)](https://github.com/huandu/go-sqlbuilder/actions)
[![GoDoc](https://godoc.org/github.com/huandu/go-sqlbuilder?status.svg)](https://pkg.go.dev/github.com/huandu/go-sqlbuilder)
[![Go Report](https://goreportcard.com/badge/github.com/huandu/go-sqlbuilder)](https://goreportcard.com/report/github.com/huandu/go-sqlbuilder)
[![Coverage Status](https://coveralls.io/repos/github/huandu/go-sqlbuilder/badge.svg?branch=master)](https://coveralls.io/github/huandu/go-sqlbuilder?branch=master)

- [Install](#install)
- [Usage](#usage)
  - [Basic usage](#basic-usage)
  - [Pre-defined SQL builders](#pre-defined-sql-builders)
  - [Build `WHERE` clause](#build-where-clause)
  - [Share `WHERE` clause among builders](#share-where-clause-among-builders)
  - [Build SQL for different systems](#build-sql-for-different-systems)
  - [Using `Struct` as a light weight ORM](#using-struct-as-a-light-weight-orm)
  - [Nested SQL](#nested-sql)
  - [Use `sql.Named` in a builder](#use-sqlnamed-in-a-builder)
  - [Argument modifiers](#argument-modifiers)
  - [Freestyle builder](#freestyle-builder)
  - [Clone builders](#clone-builders)
  - [Using special syntax to build SQL](#using-special-syntax-to-build-sql)
  - [Interpolate `args` in the `sql`](#interpolate-args-in-the-sql)
- [License](#license)

The `sqlbuilder` package offers a comprehensive suite of SQL string concatenation utilities. It is designed to facilitate the construction of SQL statements compatible with Go's standard library `sql.DB` and `sql.Stmt` interfaces, focusing on optimizing the performance of SQL statement creation and minimizing memory usage.

The primary objective of this package's design was to craft a SQL construction library that operates independently of specific database drivers and business logic. It is tailored to accommodate the diverse needs of enterprise environments, including the use of custom database drivers, adherence to specialized operational standards, integration into heterogeneous systems, and handling of non-standard SQL in intricate scenarios. Following its open-source release, the package has undergone extensive testing within a large-scale enterprise context, successfully managing the workload of hundreds of millions of orders daily and nearly ten million transactions daily, thus highlighting its robust performance and scalability.

This package is not restricted to any particular database driver and does not automatically establish connections with any database systems. It does not presuppose the execution of the generated SQL, making it versatile for a broad spectrum of application scenarios that involve the construction of SQL-like statements. It is equally well-suited for further development aimed at creating more business-specific database interaction packages, ORMs, and similar tools.

## Install

Install this package by executing the following command:

```shell
go get github.com/huandu/go-sqlbuilder
```

## Usage

### Basic usage

We can rapidly construct SQL statements using this package.

```go
sql := sqlbuilder.Select("id", "name").From("demo.user").
    Where("status = 1").Limit(10).
    String()

fmt.Println(sql)

// Output:
// SELECT id, name FROM demo.user WHERE status = 1 LIMIT 10
```

In common scenarios, it is necessary to escape all user inputs. To achieve this, initialize a builder at the outset.

```go
sb := sqlbuilder.NewSelectBuilder()

sb.Select("id", "name", sb.As("COUNT(*)", "c"))
sb.From("user")
sb.Where(sb.In("status", 1, 2, 5))

sql, args := sb.Build()
fmt.Println(sql)
fmt.Println(args)

// Output:
// SELECT id, name, COUNT(*) AS c FROM user WHERE status IN (?, ?, ?)
// [1 2 5]
```

### Pre-defined SQL builders

This package includes the following pre-defined builders. API documentation and usage examples are available in the `godoc` online documentation.

- [Struct](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Struct): Factory for creating builders based on struct definitions.
- [CreateTableBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#CreateTableBuilder): Builder for `CREATE TABLE`.
- [SelectBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#SelectBuilder): Builder for `SELECT`.
- [InsertBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#InsertBuilder): Builder for `INSERT`.
- [UpdateBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#UpdateBuilder): Builder for `UPDATE`.
- [DeleteBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#DeleteBuilder): Builder for `DELETE`.
- [UnionBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#UnionBuilder): Builder for `UNION` and `UNION ALL`.
- [CTEBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#CTEBuilder): Builder for Common Table Expression (CTE), e.g. `WITH name (col1, col2) AS (SELECT ...)`.
- [Buildf](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Buildf): Freestyle builder employing `fmt.Sprintf`-like syntax.
- [Build](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Build): Advanced freestyle builder utilizing special syntax as defined in [Args#Compile](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Args.Compile).
- [BuildNamed](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#BuildNamed): Advanced freestyle builder that uses `${key}` to reference values by key in a map.

A unique method, `SQL(sql string)`, is implemented across all statement builders, enabling the insertion of any arbitrary SQL segment into a builder during SQL construction. This feature is particularly beneficial for crafting SQL statements that incorporate non-standard syntax required by OLTP or OLAP systems.

```go
// Build a SQL to create a HIVE table.
sql := sqlbuilder.CreateTable("users").
    SQL("PARTITION BY (year)").
    SQL("AS").
    SQL(
        sqlbuilder.Select("columns[0] id", "columns[1] name", "columns[2] year").
            From("`all-users.csv`").
            String(),
    ).
    String()

fmt.Println(sql)

// Output:
// CREATE TABLE users PARTITION BY (year) AS SELECT columns[0] id, columns[1] name, columns[2] year FROM `all-users.csv`
```

Below are several utility methods designed to address special cases.

- [Flatten](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Flatten) enables the recursive conversion of an array-like variable into a flat slice of `[]interface{}`. For example, invoking `Flatten([]interface{"foo", []int{2, 3}})` yields `[]interface{}{"foo", 2, 3}`. This method is compatible with builder methods such as `In`, `NotIn`, `Values`, etc., facilitating the conversion of a typed array into `[]interface{}` or the merging of inputs.
- [List](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#List) operates similarly to `Flatten`, with the exception that its return value is specifically intended for use as builder arguments. For example, `Buildf("my_func(%v)", List([]int{1, 2, 3})).Build()` generates SQL `my_func(?, ?, ?)` with arguments `[]interface{}{1, 2, 3}`.
- [Raw](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Raw) designates a string as a "raw string" within arguments. For instance, `Buildf("SELECT %v", Raw("NOW()")).Build()` results in SQL `SELECT NOW()`.

For detailed instructions on utilizing these builders, consult the [examples provided on GoDoc](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#pkg-examples).

### Build `WHERE` clause

`WHERE` clause is the most important part of a SQL. We can use `Where` method to add one or more conditions to a builder.

To simplify the construction of `WHERE` clauses, a utility type named `Cond` is provided for condition building. All builders that support `WHERE` clauses possess an anonymous `Cond` field, enabling the invocation of `Cond` methods on these builders.

```go
sb := sqlbuilder.Select("id").From("user")
sb.Where(
    sb.In("status", 1, 2, 5),
    sb.Or(
        sb.Equal("name", "foo"),
        sb.Like("email", "foo@%"),
    ),
)

sql, args := sb.Build()
fmt.Println(sql)
fmt.Println(args)

// Output:
// SELECT id FROM user WHERE status IN (?, ?, ?) AND (name = ? OR email LIKE ?)
// [1 2 5 foo foo@%]
```

There are many methods for building conditions.

- [Cond.Equal](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.Equal)/[Cond.E](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.E)/[Cond.EQ](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.EQ): `field = value`.
- [Cond.NotEqual](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.NotEqual)/[Cond.NE](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.NE)/[Cond.NEQ](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.NEQ): `field <> value`.
- [Cond.GreaterThan](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.GreaterThan)/[Cond.G](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.G)/[Cond.GT](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.GT): `field > value`.
- [Cond.GreaterEqualThan](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.GreaterEqualThan)/[Cond.GE](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.GE)/[Cond.GTE](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.GTE): `field >= value`.
- [Cond.LessThan](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.LessThan)/[Cond.L](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.L)/[Cond.LT](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.LT): `field < value`.
- [Cond.LessEqualThan](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.LessEqualThan)/[Cond.LE](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.LE)/[Cond.LTE](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.LTE): `field <= value`.
- [Cond.In](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.In): `field IN (value1, value2, ...)`.
- [Cond.NotIn](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.NotIn): `field NOT IN (value1, value2, ...)`.
- [Cond.Like](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.Like): `field LIKE value`.
- [Cond.ILike](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.ILike): `field ILIKE value`.
- [Cond.NotLike](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.NotLike): `field NOT LIKE value`.
- [Cond.NotILike](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.NotILike): `field NOT ILIKE value`.
- [Cond.Between](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.Between): `field BETWEEN lower AND upper`.
- [Cond.NotBetween](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.NotBetween): `field NOT BETWEEN lower AND upper`.
- [Cond.IsNull](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.IsNull): `field IS NULL`.
- [Cond.IsNotNull](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.IsNotNull): `field IS NOT NULL`.
- [Cond.Exists](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.Exists): `EXISTS (subquery)`.
- [Cond.NotExists](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.NotExists): `NOT EXISTS (subquery)`.
- [Cond.Not](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.Not): `NOT expr`.
- [Cond.Any](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.Any): `field op ANY (value1, value2, ...)`.
- [Cond.All](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.All): `field op ALL (value1, value2, ...)`.
- [Cond.Some](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.Some): `field op SOME (value1, value2, ...)`.
- [Cond.IsDistinctFrom](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.IsDistinctFrom) `field IS DISTINCT FROM value`.
- [Cond.IsNotDistinctFrom](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.IsNotDistinctFrom) `field IS NOT DISTINCT FROM value`.
- [Cond.Var](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.Var): A placeholder for any value.

There are also some methods to combine conditions.

- [Cond.And](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.And): Combine conditions with `AND` operator.
- [Cond.Or](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Cond.Or): Combine conditions with `OR` operator.

### Share `WHERE` clause among builders

Due to the importance of the `WHERE` statement in SQL, we often need to continuously append conditions and even share some common `WHERE` conditions among different builders. Therefore, we abstract the `WHERE` statement into a `WhereClause` struct, which can be used to create reusable `WHERE` conditions.

The following example illustrates how to transfer a `WHERE` clause from a `SelectBuilder` to an `UpdateBuilder`.

```go
// Build a SQL to select a user from database.
sb := Select("name", "level").From("users")
sb.Where(
    sb.Equal("id", 1234),
)
fmt.Println(sb)

ub := Update("users")
ub.Set(
    ub.Add("level", 10),
)

// Set the WHERE clause of UPDATE to the WHERE clause of SELECT.
ub.WhereClause = sb.WhereClause
fmt.Println(ub)

// Output:
// SELECT name, level FROM users WHERE id = ?
// UPDATE users SET level = level + ? WHERE id = ?
```

Refer to the [WhereClause](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#WhereClause) examples to learn its usage.

### Build SQL for different systems

SQL syntax and parameter placeholders can differ across systems. To address these variations, this package introduces a concept termed "flavor".

Currently, flavors such as `MySQL`, `PostgreSQL`, `SQLite`, `SQLServer`, `CQL`, `ClickHouse`, `Presto`, `Oracle` and `Informix` are supported. Should there be a demand for additional flavors, please submit an issue or a pull request.

By default, all builders utilize `DefaultFlavor` for SQL construction, with `MySQL` as the default setting.

For greater readibility, `PostgreSQL.NewSelectBuilder()` can be used to instantiate a `SelectBuilder` with the `PostgreSQL` flavor. All builders can be created in this way.

### Using `Struct` as a light weight ORM

`Struct` encapsulates type information and struct fields, serving as a builder factory. Utilizing `Struct` methods, one can generate `SELECT`/`INSERT`/`UPDATE`/`DELETE` builders that are pre-configured for use with the struct, thereby conserving time and mitigating the risk of typographical errors in column name entries.

One can define a struct type and employ field tags to guide `Struct` in generating the appropriate builders.

```go
type ATable struct {
    Field1     string                                    // If a field doesn't has a tag, use "Field1" as column name in SQL.
    Field2     int    `db:"field2"`                      // Use "db" in field tag to set column name used in SQL.
    Field3     int64  `db:"field3" fieldtag:"foo,bar"`   // Set fieldtag to a field. We can call `WithTag` to include fields with tag or `WithoutTag` to exclude fields with tag.
    Field4     int64  `db:"field4" fieldtag:"foo"`       // If we use `s.WithTag("foo").Select("t")`, columnes of SELECT are "t.field3" and "t.field4".
    Field5     string `db:"field5" fieldas:"f5_alias"`   // Use "fieldas" in field tag to set a column alias (AS) used in SELECT.
    Ignored    int32  `db:"-"`                           // If we set field name as "-", Struct will ignore it.
    unexported int                                       // Unexported field is not visible to Struct.
    Quoted     string `db:"quoted" fieldopt:"withquote"` // Add quote to the field using back quote or double quote. See `Flavor#Quote`.
    Empty      uint   `db:"empty" fieldopt:"omitempty"`  // Omit the field in UPDATE if it is a nil or zero value.

    // The `omitempty` can be written as a function.
    // In this case, omit empty field `Tagged` when UPDATE for tag `tag1` and `tag3` but not `tag2`.
    Tagged     string `db:"tagged" fieldopt:"omitempty(tag1,tag3)" fieldtag:"tag1,tag2,tag3"`

    // By default, the `SelectFrom("t")` will add the "t." to all names of fields matched tag.
    // We can add dot to field name to disable this behavior.
    FieldWithTableAlias string `db:"m.field"`
}
```

For detailed instructions on utilizing `Struct`, refer to the [examples](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#Struct).

Furthermore, `Struct` can be employed as a zero-configuration ORM. Unlike most ORM implementations that necessitate preliminary configurations for database connectivity, `Struct` operates without any configuration, functioning seamlessly with any SQL driver compatible with `database/sql`. `Struct` does not invoke any `database/sql` APIs; it solely generates the appropriate SQL statements with arguments for `DB#Query`/`DB#Exec` or an array of struct field addresses for `Rows#Scan`/`Row#Scan`.

The following example demonstrates the use of `Struct` as an ORM. It should be relatively straightforward for developers well-versed in `database/sql` APIs.

```go
type User struct {
    ID     int64  `db:"id" fieldtag:"pk"`
    Name   string `db:"name"`
    Status int    `db:"status"`
}

// A global variable for creating SQL builders.
// All methods of userStruct are thread-safe.
var userStruct = NewStruct(new(User))

func ExampleStruct() {
    // Prepare SELECT query.
    //     SELECT user.id, user.name, user.status FROM user WHERE id = 1234
    sb := userStruct.SelectFrom("user")
    sb.Where(sb.Equal("id", 1234))

    // Execute the query and scan the results into the user struct.
    sql, args := sb.Build()
    rows, _ := db.Query(sql, args...)
    defer rows.Close()

    // Scan row data and set value to user.
    // Assuming the following data is retrieved:
    //
    //     |  id  |  name  | status |
    //     |------|--------|--------|
    //     | 1234 | huandu | 1      |
    var user User
    rows.Scan(userStruct.Addr(&user)...)

    fmt.Println(sql)
    fmt.Println(args)
    fmt.Printf("%#v", user)

    // Output:
    // SELECT user.id, user.name, user.status FROM user WHERE id = ?
    // [1234]
    // sqlbuilder.User{ID:1234, Name:"huandu", Status:1}
}
```

In numerous production environments, table column names adhere to the snake_case convention, e.g., `user_id`. Conversely, struct fields in Go are typically in CamelCase to maintain public accessibility and satisfy `golint`. Employing the `db` tag for each struct field can be redundant. To streamline this, a field mapper function can be utilized to establish a consistent rule for mapping struct field names to database column names.

The `DefaultFieldMapper` serves as a global field mapper function, tasked with the conversion of field names to a desired style. By default, it is set to `nil`, effectively performing no action. Recognizing that the majority of table column names follow the snake_case convention, one can assign `DefaultFieldMapper` to `sqlbuilder.SnakeCaseMapper`. For instances that deviate from this norm, a custom mapper can be assigned to a `Struct` via the `WithFieldMapper` method.

Here are important considerations regarding the field mapper:

- Field tag has precedence over field mapper function - thus, mapper is ignored if the `db` tag is set;
- Field mapper is called only once on a Struct when the Struct is used to create builder for the first time.

Refer to the [field mapper function sample](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#FieldMapperFunc) for an illustrative example.

### Nested SQL

Creating nested SQL is straightforward: simply use a builder as an argument for nesting.

Here is an illustrative example.

```go
sb := sqlbuilder.NewSelectBuilder()
fromSb := sqlbuilder.NewSelectBuilder()
statusSb := sqlbuilder.NewSelectBuilder()

sb.Select("id")
sb.From(sb.BuilderAs(fromSb, "user")))
sb.Where(sb.In("status", statusSb))

fromSb.Select("id").From("user").Where(fromSb.GreaterThan("level", 4))
statusSb.Select("status").From("config").Where(statusSb.Equal("state", 1))

sql, args := sb.Build()
fmt.Println(sql)
fmt.Println(args)

// Output:
// SELECT id FROM (SELECT id FROM user WHERE level > ?) AS user WHERE status IN (SELECT status FROM config WHERE state = ?)
// [4 1]
```

### Use `sql.Named` in a builder

The `sql.Named` function, as defined in the `database/sql` package, facilitates the creation of named arguments within SQL statements. This feature is essential for scenarios where an argument needs to be reused multiple times within a single SQL statement. Incorporating named arguments into a builder is straightforward: treat them as regular arguments.

Here is a sample.

```go
now := time.Now().Unix()
start := sql.Named("start", now-86400)
end := sql.Named("end", now+86400)
sb := sqlbuilder.NewSelectBuilder()

sb.Select("name")
sb.From("user")
sb.Where(
    sb.Between("created_at", start, end),
    sb.GE("modified_at", start),
)

sql, args := sb.Build()
fmt.Println(sql)
fmt.Println(args)

// Output:
// SELECT name FROM user WHERE created_at BETWEEN @start AND @end AND modified_at >= @start
// [{{} start 1514458225} {{} end 1514544625}]
```

### Argument modifiers

Several argument modifiers are available:

- `List(arg)` encapsulates a series of arguments. Given `arg` as a slice or array, for instance, a slice containing three integers, it compiles to `?, ?, ?` and is presented in the final arguments as three individual integers. This serves as a convenience tool, utilizable within `IN` expressions or within the `VALUES` clause of an `INSERT INTO` statement.
- `TupleNames(names)` and `Tuple(values)` facilitate the representation of tuple syntax in SQL. For usage examples, refer to [Tuple](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#example-Tuple).
- `Named(name, arg)` designates a named argument. Functionality is limited to `Build` or `BuildNamed`, where it defines a named placeholder using the syntax `${name}`.
- `Raw(expr)` designates `expr` as a plain string within SQL, as opposed to an argument. During the construction of a builder, raw expressions are directly embedded into the SQL string, omitting the need for `?` placeholders.

### Freestyle builder

A builder essentially serves as a means to log arguments. For constructing lengthy SQL statements that incorporate numerous special syntax elements (e.g., special comments intended for a database proxy), `Buildf` can be employed to format the SQL string using a syntax akin to `fmt.Sprintf`.

```go
sb := sqlbuilder.NewSelectBuilder()
sb.Select("id").From("user")

explain := sqlbuilder.Buildf("EXPLAIN %v LEFT JOIN SELECT * FROM banned WHERE state IN (%v, %v)", sb, 1, 2)
sql, args := explain.Build()
fmt.Println(sql)
fmt.Println(args)

// Output:
// EXPLAIN SELECT id FROM user LEFT JOIN SELECT * FROM banned WHERE state IN (?, ?)
// [1 2]
```

### Clone builders

The `Clone` methods make any builder reusable as a template. You can create a partially initialized builder once (even as a global), then call `Clone()` to get an independent copy to customize per request. This avoids repeated setup while keeping shared templates immutable and safe for concurrent use.

Supported builders with `Clone`:

- [CreateTableBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#CreateTableBuilder)
- [CTEBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#CTEBuilder)
- [CTEQueryBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#CTEQueryBuilder)
- [DeleteBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#DeleteBuilder)
- [InsertBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#InsertBuilder)
- [SelectBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#SelectBuilder)
- [UnionBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#UnionBuilder)
- [UpdateBuilder](https://pkg.go.dev/github.com/huandu/go-sqlbuilder#UpdateBuilder)

Example: define a global SELECT template and clone it per call

```go
package yourpkg

import "github.com/huandu/go-sqlbuilder"

// Global template — safe to reuse by cloning.
var baseUserSelect = sqlbuilder.NewSelectBuilder().
    Select("id", "name", "email").
    From("users").
    Where("deleted_at IS NULL")

func ListActiveUsers(limit, offset int) (string, []interface{}) {
    sb := baseUserSelect.Clone() // independent copy
    sb.OrderBy("id").Asc()
    sb.Limit(limit).Offset(offset)
    return sb.Build()
}

func GetActiveUserByID(id int64) (string, []interface{}) {
    sb := baseUserSelect.Clone() // start from the same template
    sb.Where(sb.Equal("id", id))
    sb.Limit(1)
    return sb.Build()
}
```

The same template pattern applies to other builders. For example, keep a base `UpdateBuilder` with the table and common `SET` clauses, or a base `CTEBuilder` defining reusable CTEs, then `Clone()` and add query-specific `WHERE`/`ORDER BY`/`LIMIT`/`RETURNING` as needed.

### Using special syntax to build SQL

The `sqlbuilder` package incorporates special syntax for representing uncompiled SQL internally. To leverage this syntax for developing customized tools, the `Build` function can be utilized to compile it with the necessary arguments.

The format string employs special syntax for representing arguments:

- `$?` references successive arguments supplied in the function call, functioning similarly to `%v` in `fmt.Sprintf`.
- `$0`, `$1`, ..., `$n` reference the nth argument provided in the call; subsequent `$?` will then refer to arguments n+1 onwards.
- `${name}` references a named argument defined by `Named` using the specified `name`.
- `$$` represents a literal `"$"` character.

```go
sb := sqlbuilder.NewSelectBuilder()
sb.Select("id").From("user").Where(sb.In("status", 1, 2))

b := sqlbuilder.Build("EXPLAIN $? LEFT JOIN SELECT * FROM $? WHERE created_at > $? AND state IN (${states}) AND modified_at BETWEEN $2 AND $?",
    sb, sqlbuilder.Raw("banned"), 1514458225, 1514544625, sqlbuilder.Named("states", sqlbuilder.List([]int{3, 4, 5})))
sql, args := b.Build()

fmt.Println(sql)
fmt.Println(args)

// Output:
// EXPLAIN SELECT id FROM user WHERE status IN (?, ?) LEFT JOIN SELECT * FROM banned WHERE created_at > ? AND state IN (?, ?, ?) AND modified_at BETWEEN ? AND ?
// [1 2 1514458225 3 4 5 1514458225 1514544625]
```

For scenarios where only the `${name}` syntax is required to reference named arguments, utilize `BuildNamed`. This function disables all special syntax except for `${name}` and `$$`.

### Interpolate `args` in the `sql`

Certain SQL-like drivers, such as those for Redis or Elasticsearch, do not implement the `StmtExecContext#ExecContext` method. These drivers encounter issues when `len(args) > 0`. The sole workaround is to interpolate `args` directly into the `sql` string and then execute the resulting query with the driver.

The interpolation feature in this package is designed to provide a "basically sufficient" level of functionality, rather than a capability that rivals the comprehensive features of various SQL drivers and DBMS systems.

_Security warning_: While efforts are made to escape special characters in interpolation methods, this approach remains less secure than using `Stmt` as implemented by SQL drivers.

This feature draws inspiration from the interpolation capabilities found in the `github.com/go-sql-driver/mysql` package.

Here is an example specifically for MySQL:

```go
sb := MySQL.NewSelectBuilder()
sb.Select("name").From("user").Where(
    sb.NE("id", 1234),
    sb.E("name", "Charmy Liu"),
    sb.Like("desc", "%mother's day%"),
)
sql, args := sb.Build()
query, err := MySQL.Interpolate(sql, args)

fmt.Println(query)
fmt.Println(err)

// Output:
// SELECT name FROM user WHERE id <> 1234 AND name = 'Charmy Liu' AND desc LIKE '%mother\'s day%'
// <nil>
```

Here is an example for PostgreSQL, noting that dollar quoting is supported:

```go
// Only the last `$1` is interpolated.
// Others are not interpolated as they are inside dollar quote (the `$$`).
query, err := PostgreSQL.Interpolate(`
CREATE FUNCTION dup(in int, out f1 int, out f2 text) AS $$
    SELECT $1, CAST($1 AS text) || ' is text'
$$
LANGUAGE SQL;

SELECT * FROM dup($1);`, []interface{}{42})

fmt.Println(query)
fmt.Println(err)

// Output:
//
// CREATE FUNCTION dup(in int, out f1 int, out f2 text) AS $$
//     SELECT $1, CAST($1 AS text) || ' is text'
// $$
// LANGUAGE SQL;
//
// SELECT * FROM dup(42);
// <nil>
```

## License

This package is licensed under the MIT license. For more information, refer to the LICENSE file.
