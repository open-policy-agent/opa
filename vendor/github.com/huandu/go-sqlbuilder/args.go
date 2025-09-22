// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/huandu/go-clone"
)

// Args stores arguments associated with a SQL.
type Args struct {
	// The default flavor used by `Args#Compile`
	Flavor Flavor

	indexBase    int
	argValues    *valueStore
	namedArgs    map[string]int
	sqlNamedArgs map[string]int
	onlyNamed    bool
}

func init() {
	// Predefine some $n args to avoid additional memory allocation.
	predefinedArgs = make([]string, 0, maxPredefinedArgs)

	for i := 0; i < maxPredefinedArgs; i++ {
		predefinedArgs = append(predefinedArgs, fmt.Sprintf("$%v", i))
	}
}

const maxPredefinedArgs = 64

var predefinedArgs []string

// Add adds an arg to Args and returns a placeholder.
func (args *Args) Add(arg interface{}) string {
	idx := args.add(arg)

	if idx < maxPredefinedArgs {
		return predefinedArgs[idx]
	}

	return fmt.Sprintf("$%v", idx)
}

func (args *Args) add(arg interface{}) int {
	idx := args.argValues.Len() + args.indexBase

	switch a := arg.(type) {
	case sql.NamedArg:
		if args.sqlNamedArgs == nil {
			args.sqlNamedArgs = map[string]int{}
		}

		if p, ok := args.sqlNamedArgs[a.Name]; ok {
			arg = args.argValues.Load(p)
			break
		}

		args.sqlNamedArgs[a.Name] = idx
	case namedArgs:
		if args.namedArgs == nil {
			args.namedArgs = map[string]int{}
		}

		if p, ok := args.namedArgs[a.name]; ok {
			arg = args.argValues.Load(p)
			break
		}

		// Find out the real arg and add it to args.
		idx = args.add(a.arg)
		args.namedArgs[a.name] = idx
		return idx
	}

	if args.argValues == nil {
		args.argValues = &valueStore{}
	}

	args.argValues.Add(arg)
	return idx
}

// Replace replaces the placeholder with arg.
//
// The placeholder must be the value returned by `Add`, e.g. "$1".
// If the placeholder is not found, this method does nothing.
func (args *Args) Replace(placeholder string, arg interface{}) {
	dollar := strings.IndexRune(placeholder, '$')

	if dollar != 0 {
		return
	}

	if i, err := strconv.Atoi(placeholder[1:]); err == nil {
		i -= args.indexBase
		args.argValues.Set(i, arg)
	}
}

// Compile compiles builder's format to standard sql and returns associated args.
//
// The format string uses a special syntax to represent arguments.
//
//	$? refers successive arguments passed in the call. It works similar as `%v` in `fmt.Sprintf`.
//	$0 $1 ... $n refers nth-argument passed in the call. Next $? will use arguments n+1.
//	${name} refers a named argument created by `Named` with `name`.
//	$$ is a "$" string.
func (args *Args) Compile(format string, initialValue ...interface{}) (query string, values []interface{}) {
	return args.CompileWithFlavor(format, args.Flavor, initialValue...)
}

// CompileWithFlavor compiles builder's format to standard sql with flavor and returns associated args.
//
// See doc for `Compile` to learn details.
func (args *Args) CompileWithFlavor(format string, flavor Flavor, initialValue ...interface{}) (query string, values []interface{}) {
	idx := strings.IndexRune(format, '$')
	offset := 0
	ctx := &argsCompileContext{
		stringBuilder: newStringBuilder(),
		Flavor:        flavor,
		Values:        initialValue,
	}

	if ctx.Flavor == invalidFlavor {
		ctx.Flavor = DefaultFlavor
	}

	for idx >= 0 && len(format) > 0 {
		if idx > 0 {
			ctx.WriteString(format[:idx])
		}

		format = format[idx+1:]

		// Treat the $ at the end of format is a normal $ rune.
		if len(format) == 0 {
			ctx.WriteRune('$')
			break
		}

		if r := format[0]; r == '$' {
			ctx.WriteRune('$')
			format = format[1:]
		} else if r == '{' {
			format = args.compileNamed(ctx, format)
		} else if !args.onlyNamed && '0' <= r && r <= '9' {
			format, offset = args.compileDigits(ctx, format, offset)
		} else if !args.onlyNamed && r == '?' {
			format, offset = args.compileSuccessive(ctx, format[1:], offset)
		} else {
			// For unknown $ expression format, treat it as a normal $ rune.
			ctx.WriteRune('$')
		}

		idx = strings.IndexRune(format, '$')
	}

	if len(format) > 0 {
		ctx.WriteString(format)
	}

	query = ctx.String()
	values = args.mergeSQLNamedArgs(ctx)
	return
}

// Value returns the value of the arg.
// The arg must be the value returned by `Add`.
func (args *Args) Value(arg string) interface{} {
	_, values := args.Compile(arg)

	if len(values) == 0 {
		return nil
	}

	return values[0]
}

func (args *Args) compileNamed(ctx *argsCompileContext, format string) string {
	i := 1

	for ; i < len(format) && format[i] != '}'; i++ {
		// Nothing.
	}

	// Invalid $ format. Ignore it.
	if i == len(format) {
		return format
	}

	name := format[1:i]
	format = format[i+1:]

	if p, ok := args.namedArgs[name]; ok {
		format, _ = args.compileSuccessive(ctx, format, p-args.indexBase)
	}

	return format
}

func (args *Args) compileDigits(ctx *argsCompileContext, format string, offset int) (string, int) {
	i := 1

	for ; i < len(format) && '0' <= format[i] && format[i] <= '9'; i++ {
		// Nothing.
	}

	digits := format[:i]
	format = format[i:]

	if pointer, err := strconv.Atoi(digits); err == nil {
		return args.compileSuccessive(ctx, format, pointer-args.indexBase)
	}

	return format, offset
}

func (args *Args) compileSuccessive(ctx *argsCompileContext, format string, offset int) (string, int) {
	if offset < 0 || offset >= args.argValues.Len() {
		ctx.WriteString("/* INVALID ARG $")
		ctx.WriteString(strconv.Itoa(offset))
		ctx.WriteString(" */")
		return format, offset
	}

	arg := args.argValues.Load(offset)
	ctx.WriteValue(arg)

	return format, offset + 1
}

func (args *Args) mergeSQLNamedArgs(ctx *argsCompileContext) []interface{} {
	if len(args.sqlNamedArgs) == 0 && len(ctx.NamedArgs) == 0 {
		return ctx.Values
	}

	values := ctx.Values
	existingNames := make(map[string]struct{}, len(ctx.NamedArgs))

	// Add all named args to values.
	// Remove duplicated named args in this step.
	for _, arg := range ctx.NamedArgs {
		if _, ok := existingNames[arg.Name]; !ok {
			existingNames[arg.Name] = struct{}{}
			values = append(values, arg)
		}
	}

	// Stabilize the sequence to make it easier to write test cases.
	ints := make([]int, 0, len(args.sqlNamedArgs))

	for n, p := range args.sqlNamedArgs {
		if _, ok := existingNames[n]; ok {
			continue
		}

		ints = append(ints, p)
	}

	sort.Ints(ints)

	for _, i := range ints {
		values = append(values, args.argValues.Load(i))
	}

	return values
}

func parseNamedArgs(initialValue []interface{}) (values []interface{}, namedValues []sql.NamedArg) {
	if len(initialValue) == 0 {
		values = initialValue
		return
	}

	// sql.NamedArgs must be placed at the end of the initial value.
	size := len(initialValue)
	i := size

	for ; i > 0; i-- {
		switch initialValue[i-1].(type) {
		case sql.NamedArg:
			continue
		}

		break
	}

	if i == size {
		values = initialValue
		return
	}

	values = initialValue[:i]
	namedValues = make([]sql.NamedArg, 0, size-i)

	for ; i < size; i++ {
		namedValues = append(namedValues, initialValue[i].(sql.NamedArg))
	}

	return
}

type argsCompileContext struct {
	*stringBuilder

	Flavor    Flavor
	Values    []interface{}
	NamedArgs []sql.NamedArg
}

func (ctx *argsCompileContext) WriteValue(arg interface{}) {
	switch a := arg.(type) {
	case Builder:
		s, values := a.BuildWithFlavor(ctx.Flavor, ctx.Values...)
		ctx.WriteString(s)

		// Add all values to ctx.
		// Named args must be located at the end of values.
		values, namedArgs := parseNamedArgs(values)
		ctx.Values = values
		ctx.NamedArgs = append(ctx.NamedArgs, namedArgs...)

	case sql.NamedArg:
		ctx.WriteRune('@')
		ctx.WriteString(a.Name)
		ctx.NamedArgs = append(ctx.NamedArgs, a)

	case rawArgs:
		ctx.WriteString(a.expr)

	case listArgs:
		if a.isTuple {
			ctx.WriteRune('(')
		}

		if len(a.args) > 0 {
			ctx.WriteValue(a.args[0])
		}

		for i := 1; i < len(a.args); i++ {
			ctx.WriteString(", ")
			ctx.WriteValue(a.args[i])
		}

		if a.isTuple {
			ctx.WriteRune(')')
		}

	case condBuilder:
		a.Builder(ctx)

	default:
		switch ctx.Flavor {
		case MySQL, SQLite, CQL, ClickHouse, Presto, Informix, Doris:
			ctx.WriteRune('?')
		case PostgreSQL:
			fmt.Fprintf(ctx, "$%d", len(ctx.Values)+1)
		case SQLServer:
			fmt.Fprintf(ctx, "@p%d", len(ctx.Values)+1)
		case Oracle:
			fmt.Fprintf(ctx, ":%d", len(ctx.Values)+1)
		default:
			panic(fmt.Errorf("Args.CompileWithFlavor: invalid flavor %v (%v)", ctx.Flavor, int(ctx.Flavor)))
		}

		ctx.Values = append(ctx.Values, arg)
	}
}

func (ctx *argsCompileContext) WriteValues(values []interface{}, sep string) {
	if len(values) == 0 {
		return
	}

	ctx.WriteValue(values[0])

	for _, v := range values[1:] {
		ctx.WriteString(sep)
		ctx.WriteValue(v)
	}
}

type valueStore struct {
	Values []interface{}
}

func init() {
	// The values in valueStore should be shadow-copied to avoid unnecessary cost.
	t := reflect.TypeOf(valueStore{})
	clone.SetCustomFunc(t, func(allocator *clone.Allocator, old, new reflect.Value) {
		values := old.FieldByName("Values")
		newValues := allocator.Clone(values)
		new.FieldByName("Values").Set(newValues)
	})
}

func (as *valueStore) Len() int {
	if as == nil {
		return 0
	}

	return len(as.Values)
}

// Add adds an arg to argsValues and returns its index.
func (as *valueStore) Add(arg interface{}) int {
	as.Values = append(as.Values, arg)
	return len(as.Values) - 1
}

// Set sets the arg value by index.
func (as *valueStore) Set(index int, arg interface{}) {
	if as == nil || index < 0 || index >= len(as.Values) {
		return
	}

	as.Values[index] = arg
}

// Load returns the arg value by index.
// Returns nil if index is out of range or as itself is nil.
func (as *valueStore) Load(index int) interface{} {
	if as == nil || index < 0 || index >= len(as.Values) {
		return nil
	}

	return as.Values[index]
}
