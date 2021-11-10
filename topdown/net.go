package topdown

import (
	"errors"
	"fmt"
	"net"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

type lookupIPAddrCacheKey string

func builtinLookupIPAddr(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	name, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	key := lookupIPAddrCacheKey(name)
	if val, ok := bctx.Cache.Get(key); ok {
		return iter(val.(*ast.Term))
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(bctx.Context, string(name))
	if err != nil {
		var derr *net.DNSError
		if errors.As(err, &derr) && derr.Temporary() {
			return Halt{
				Err: &Error{
					Code:    BuiltinErr,
					Message: fmt.Sprintf("%s: %s", ast.NetLookupIPAddr.Name, derr.Error()),
				},
			}
		}
		return err
	}

	ret := ast.NewSet()
	for _, a := range addrs {
		ret.Add(ast.StringTerm(a.String()))

	}
	t := ast.NewTerm(ret)
	bctx.Cache.Put(key, t)
	return iter(t)
}

func init() {
	RegisterBuiltinFunc(ast.NetLookupIPAddr.Name, builtinLookupIPAddr)
}
