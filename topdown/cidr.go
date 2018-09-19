package topdown

import (
	"fmt"
	"net"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

func builtinNetCIDROverlap(a, b ast.Value) (ast.Value, error) {
	pattern, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}

	match, err := builtins.StringOperand(b, 1)
	if err != nil {
		return nil, err
	}

	_, cidrnet, err := net.ParseCIDR(string(pattern))
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(string(match))
	if ip == nil {
		return nil, fmt.Errorf("not a valid textual representation of an IP address: %s", string(match))
	}

	return ast.Boolean(cidrnet.Contains(ip)), nil
}

func init() {
	RegisterFunctionalBuiltin2(ast.NetCIDROverlap.Name, builtinNetCIDROverlap)
}
