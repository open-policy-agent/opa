package wazero

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/open-policy-agent/opa/topdown/cache"
	"github.com/open-policy-agent/opa/topdown/print"
	"io"
	"log"
	"strconv"
	"time"
)

func newBuiltinTable(mod Module) map[int32]topdown.BuiltinFunc {
	builtinStrAddr := mod.builtins(mod.ctx)
	builtinsJSON, err := mod.jsonDump(mod.ctx, uint64(builtinStrAddr))
	if err != nil {
		panic(err)
	}
	builtinStr := mod.readStr(uint32(builtinsJSON[0]))
	builtinNameMap := parseJsonString(builtinStr)
	builtinIdMap, err := getFuncs(builtinNameMap)
	if err != nil {
		panic(err)
	}
	log.Println(builtinIdMap)
	return builtinIdMap
}
func parseJsonString(str string) map[string]int32 {
	currKey := ""
	inKey := false
	inVal := false
	currVal := ""
	out := map[string]int32{}
	for _, char := range str {
		switch char {
		case '"':
			inKey = !inKey
		case '{':
		case '}':
			val, _ := strconv.ParseInt(currVal, 10, 32)
			out[currKey] = int32(val)
		case ':':
			inVal = true
		case ',':
			val, _ := strconv.ParseInt(currVal, 10, 32)
			out[currKey] = int32(val)
			inVal = false
			currVal = ""
			currKey = ""
		default:
			if inKey {
				currKey += string(char)
			} else if inVal {
				currVal += string(char)
			}
		}

	}
	return out
}
func getFuncs(ids map[string]int32) (map[int32]topdown.BuiltinFunc, error) {
	out := map[int32]topdown.BuiltinFunc{}
	for name, id := range ids {
		out[id] = topdown.GetBuiltin(name)
		log.Println(name)
		if out[id] == nil && name != "" {
			return out, fmt.Errorf("no function named %s", name)
		}
	}
	return out, nil
}

type exports struct {
	val_dump  func(context.Context, ...uint64) ([]uint64, error)
	val_parse func(context.Context, ...uint64) ([]uint64, error)
	json_dump func(context.Context, ...uint64) ([]uint64, error)
	malloc    func(context.Context, ...uint64) ([]uint64, error)
}
type builtinContainer struct {
	builtinIdMap map[int32]topdown.BuiltinFunc
	module       Module
	e            exports
	ctx          *topdown.BuiltinContext
}

func (b *builtinContainer) Call(args ...int32) int32 {
	log.Println("calling")
	var output *ast.Term
	pArgs := []*ast.Term{}
	for _, ter := range args[2:] {
		log.Println("value_dump")
		serialized, err := b.e.val_dump(b.module.ctx, uint64(ter))
		log.Println("post_value_dump")
		if err != nil {
			log.Println("93", err)
			panic(builtinError{err: err})
		}
		log.Println("readstr")
		data := b.module.readStr(uint32(serialized[0]))
		log.Println("ParseTerm")
		pTer, err := ast.ParseTerm(string(data))
		if err != nil {
			log.Println("99", err)
			panic(builtinError{err: err})
		}
		pArgs = append(pArgs, pTer)
	}
	log.Println("testing")
	err := b.builtinIdMap[args[0]](*b.ctx, pArgs, func(t *ast.Term) error {
		output = t
		log.Println(t)
		log.Println("hi")
		return nil
	})
	if err != nil {
		if errors.As(err, &topdown.Halt{}) {
			var e *topdown.Error
			if errors.As(err, &e) && e.Code == topdown.CancelErr {
				log.Println("112", e.Message)
				panic(cancelledError{message: e.Message})
			}
			log.Println("115", err)
			panic(builtinError{err: err})
		}
		// non-halt errors are treated as undefined ("non-strict eval" is the only
		// mode in wasm), the `output == nil` case below will return NULL
	}
	if output == nil {
		log.Println("Hi")
		return 0
	}
	outB := []byte(output.String())
	loc := b.module.writeMem(outB)
	addr, err := b.e.val_dump(b.module.ctx, uint64(loc), uint64(len(outB)))
	if err != nil {
		log.Println("128", err)
		panic(err)
	}
	log.Println("hi", addr)
	log.Println("hi", b.module.readUntil(int32(addr[0]), 0b0))
	return int32(addr[0])
}

// Exported to wasm and executes Call with the given builtin_id and arguments
func (b *builtinContainer) C0(id, ctx int32) int32 {
	return b.Call(id, ctx)
}
func (b *builtinContainer) C1(id, ctx, a1 int32) int32 {
	return b.Call(id, ctx, a1)
}
func (b *builtinContainer) C2(id, ctx, a1, a2 int32) int32 {
	return b.Call(id, ctx, a1, a2)
}
func (b *builtinContainer) C3(id, ctx, a1, a2, a3 int32) int32 {
	return b.Call(id, ctx, a1, a2, a3)
}
func (b *builtinContainer) C4(id, ctx, a1, a2, a3, a4 int32) int32 {
	return b.Call(id, ctx, a1, a2, a3, a4)
}
func (b *builtinContainer) Reset(ctx context.Context,
	seed io.Reader,
	ns time.Time,
	iqbCache cache.InterQueryCache,
	ph print.Hook,
	capabilities *ast.Capabilities) {
	if ns.IsZero() {
		ns = time.Now()
	}
	if seed == nil {
		seed = rand.Reader
	}
	b.ctx = &topdown.BuiltinContext{
		Context:                ctx,
		Metrics:                metrics.New(),
		Seed:                   seed,
		Time:                   ast.NumberTerm(json.Number(strconv.FormatInt(ns.UnixNano(), 10))),
		Cancel:                 topdown.NewCancel(),
		Runtime:                nil,
		Cache:                  make(builtins.Cache),
		Location:               nil,
		Tracers:                nil,
		QueryTracers:           nil,
		QueryID:                0,
		ParentID:               0,
		InterQueryBuiltinCache: iqbCache,
		PrintHook:              ph,
		Capabilities:           capabilities,
	}

}
func newBuiltinContainer(m Module) *builtinContainer {
	bc := builtinContainer{}
	bc.builtinIdMap = newBuiltinTable(m)
	bc.e = exports{val_dump: m.module.ExportedFunction("opa_value_dump").Call, val_parse: m.module.ExportedFunction("opa_value_parse").Call, json_dump: m.module.ExportedFunction("opa_json_dump").Call, malloc: m.module.ExportedFunction("opa_malloc").Call}
	bc.ctx = &topdown.BuiltinContext{
		Context:      m.ctx,
		Metrics:      metrics.New(),
		Seed:         rand.Reader,
		Time:         ast.NumberTerm(json.Number(strconv.FormatInt(time.Now().UnixNano(), 10))),
		Cancel:       topdown.NewCancel(),
		Runtime:      nil,
		Cache:        make(builtins.Cache),
		Location:     nil,
		Tracers:      nil,
		QueryTracers: nil,
		QueryID:      0,
		ParentID:     0,
	}
	return &bc
}
